<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import { GetScreenshotHistory, GetScreenshotByID, ClearScreenshotHistory, SaveScreenshotToFile, GetScreenshotSaveDir } from '../../wailsjs/go/app/App';
  import { SendCursorMove, SendCursorShow, SendCursorHide, SendCursorClick } from '../../wailsjs/go/app/App';
  import { SendHint, ClearHints, SendTextOverlay, ClearTextOverlays } from '../../wailsjs/go/app/App';
  import { SendDrawStart, SendDrawMove, SendDrawEnd, SendDrawClear, SendDrawUndo } from '../../wailsjs/go/app/App';

  export let isConnected: boolean;
  export let screenshotData: {id?: number, data: string, width: number, height: number, timestamp?: string} | null = null;
  export let role: string;
  export let history: HistoryEntry[] = [];
  export let historyLimit: number = 50;

  const dispatch = createEventDispatcher();

  let saving = false;
  let saveSuccess = false;

  export type HistoryEntry = {
    id: number;
    timestamp: string;
    width: number;
    height: number;
    size: number;
    data?: string;
  };

  let selectedId: number | null = null;
  let loadingId: number | null = null;

  // === View mode: 'gallery' | 'editor' | 'fullscreen' ===
  let viewMode: 'gallery' | 'editor' | 'fullscreen' = 'gallery';
  let editorScreenshot: {id?: number, data: string, width: number, height: number, timestamp?: string} | null = null;

  // === Tool state ===
  type ToolMode = 'none' | 'select' | 'cursor' | 'hint' | 'text' | 'brush' | 'eraser' | 'arrow' | 'rect' | 'circle' | 'oval' | 'line' | 'checkmark' | 'cross';
  let activeTool: ToolMode = 'none';
  let drawColor = '#FF0000';
  let drawThickness = 0.003;
  let isDrawing = false;
  let hintInputVisible = false;
  let hintInputX = 0;
  let hintInputY = 0;
  let hintRelX = 0;
  let hintRelY = 0;
  let hintText = '';
  let hintInputEl: HTMLInputElement | null = null;

  let textInputVisible = false;
  let textInputX = 0;
  let textInputY = 0;
  let textRelX = 0;
  let textRelY = 0;
  let textInputValue = '';
  let textInputEl: HTMLInputElement | null = null;

  let screenshotImgEl: HTMLImageElement | null = null;

  // === Local drawing canvas (Phase 2) ===
  let drawCanvasEl: HTMLCanvasElement | null = null;
  let drawCtx: CanvasRenderingContext2D | null = null;

  // Stroke types for local drawing
  interface StrokeData {
    tool: string;
    color: string;
    thickness: number;
    points: {x: number, y: number}[];
    startX: number;
    startY: number;
    endX?: number;
    endY?: number;
    // Interactive shape properties (Phase 3)
    offsetX?: number;
    offsetY?: number;
    scaleX?: number;
    scaleY?: number;
    selected?: boolean;
  }

  let localStrokes: StrokeData[] = [];
  let currentLocalStroke: StrokeData | null = null;
  let drawStartCoords: {x: number, y: number} | null = null;

  // === Interactive shapes (Phase 3) ===
  let selectedShapeIndex: number = -1;
  let isDraggingShape = false;
  let isResizingShape = false;
  let resizeHandle: string = '';
  let dragStartX = 0;
  let dragStartY = 0;
  let dragOrigShape: StrokeData | null = null;
  const HANDLE_SIZE = 8;

  // === Local ghost cursor for controller (so they see where they point) ===
  let localCursorPos: {x: number, y: number} | null = null;
  let cursorRipples: {id: number, x: number, y: number}[] = [];
  let rippleIdCounter = 0;

  // Throttle for cursor/draw moves
  let lastMoveTime = 0;
  const MOVE_THROTTLE_MS = 16; // ~60fps

  const COLORS = ['#FF0000', '#FF6600', '#FFCC00', '#00CC00', '#0088FF', '#AA00FF', '#FFFFFF', '#000000'];

  const DRAW_TOOLS: ToolMode[] = ['brush', 'eraser', 'arrow', 'rect', 'circle', 'oval', 'line', 'checkmark', 'cross'];
  const isDrawTool = (t: ToolMode) => DRAW_TOOLS.includes(t);

  // When new screenshot arrives, add to local history
  $: if (screenshotData && screenshotData.id) {
    addToLocalHistory(screenshotData);
    selectedId = screenshotData.id;
  }

  function addToLocalHistory(data: {id?: number, data: string, width: number, height: number, timestamp?: string}) {
    if (!data.id) return;
    const existing = history.find(h => h.id === data.id);
    if (existing) {
      existing.data = data.data;
      history = [...history];
      dispatch('historyUpdate', history);
      return;
    }
    const entry: HistoryEntry = {
      id: data.id,
      timestamp: data.timestamp || new Date().toISOString(),
      width: data.width,
      height: data.height,
      size: Math.round(data.data.length * 0.75),
      data: data.data
    };
    history = [...history, entry];
    const maxItems = historyLimit > 0 ? historyLimit : 50;
    if (history.length > maxItems) {
      history = history.slice(-maxItems);
    }
    dispatch('historyUpdate', history);
  }

  async function selectScreenshot(id: number) {
    if (loadingId === id) return;
    selectedId = id;
    const entry = history.find(h => h.id === id);
    if (entry && entry.data) {
      screenshotData = { id: entry.id, data: entry.data, width: entry.width, height: entry.height, timestamp: entry.timestamp };
      return;
    }
    loadingId = id;
    try {
      const result = await GetScreenshotByID(id);
      if (result) {
        screenshotData = { id: result.id, data: result.data, width: result.width, height: result.height };
        if (entry) entry.data = result.data;
      }
    } catch (e) {
      console.error('Failed to load screenshot:', e);
    } finally {
      loadingId = null;
    }
  }

  async function clearHistory() {
    try {
      await ClearScreenshotHistory();
      history = [];
      selectedId = null;
      dispatch('historyUpdate', history);
    } catch (e) {
      console.error('Failed to clear history:', e);
    }
  }

  async function saveScreenshot() {
    if (!screenshotData || saving) return;
    saving = true;
    saveSuccess = false;
    try {
      const filename = `screenshot-${new Date().toISOString().slice(0, 19).replace(/:/g, '-')}.jpg`;
      const savedPath = await SaveScreenshotToFile(screenshotData.data, filename);
      saveSuccess = true;
      dispatch('log', { level: 'info', message: `Скриншот сохранен: ${savedPath}` });
      setTimeout(() => { saveSuccess = false; }, 2000);
    } catch (e: any) {
      console.error('Failed to save screenshot:', e);
      dispatch('log', { level: 'error', message: `Ошибка сохранения: ${e.message || e}` });
    } finally {
      saving = false;
    }
  }

  function formatTime(timestamp: string): string {
    try { return new Date(timestamp).toLocaleTimeString(); } catch { return ''; }
  }

  function formatSize(bytes: number): string {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
  }

  // === Editor mode ===
  function openEditor(data: typeof screenshotData) {
    if (!data) return;
    editorScreenshot = data;
    viewMode = 'editor';
    activeTool = 'none';
    localStrokes = [];
    selectedShapeIndex = -1;
    requestAnimationFrame(syncCanvasSize);
  }

  function openFullscreen(data: typeof screenshotData) {
    if (!data) return;
    editorScreenshot = data;
    viewMode = 'fullscreen';
    activeTool = 'none';
    localStrokes = [];
    selectedShapeIndex = -1;
    requestAnimationFrame(syncCanvasSize);
  }

  function closeEditor() {
    deactivateTool();
    viewMode = 'gallery';
    editorScreenshot = null;
    localStrokes = [];
    selectedShapeIndex = -1;
  }

  function exitFullscreen() {
    viewMode = 'editor';
    requestAnimationFrame(syncCanvasSize);
  }

  // === Canvas sync ===
  function syncCanvasSize() {
    if (!drawCanvasEl || !screenshotImgEl) return;
    const rect = screenshotImgEl.getBoundingClientRect();
    const dpr = window.devicePixelRatio || 1;
    drawCanvasEl.width = rect.width * dpr;
    drawCanvasEl.height = rect.height * dpr;
    drawCanvasEl.style.width = rect.width + 'px';
    drawCanvasEl.style.height = rect.height + 'px';
    drawCtx = drawCanvasEl.getContext('2d');
    if (drawCtx) {
      drawCtx.setTransform(dpr, 0, 0, dpr, 0, 0);
    }
    redrawLocalCanvas();
  }

  // === Tool functions ===
  function setTool(tool: ToolMode) {
    if (activeTool === tool) {
      deactivateTool();
      return;
    }
    if (activeTool === 'cursor') {
      SendCursorHide();
      localCursorPos = null;
      cursorRipples = [];
    }
    selectedShapeIndex = -1;
    activeTool = tool;
    hintInputVisible = false;
    textInputVisible = false;
    if (tool === 'cursor') {
      SendCursorShow();
    }
    redrawLocalCanvas();
  }

  function deactivateTool() {
    if (activeTool === 'cursor') {
      SendCursorHide();
      localCursorPos = null;
      cursorRipples = [];
    }
    activeTool = 'none';
    selectedShapeIndex = -1;
    hintInputVisible = false;
    textInputVisible = false;
    redrawLocalCanvas();
  }

  function getRelativeCoords(e: MouseEvent): {x: number, y: number} | null {
    if (!screenshotImgEl) return null;
    const rect = screenshotImgEl.getBoundingClientRect();
    const x = (e.clientX - rect.left) / rect.width;
    const y = (e.clientY - rect.top) / rect.height;
    if (x < 0 || x > 1 || y < 0 || y > 1) return null;
    return { x, y };
  }

  function getCanvasCoords(e: MouseEvent): {x: number, y: number} | null {
    if (!screenshotImgEl) return null;
    const rect = screenshotImgEl.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;
    if (x < 0 || x > rect.width || y < 0 || y > rect.height) return null;
    return { x, y };
  }

  // === Local drawing functions (Phase 2) ===
  function toCanvasPixel(relX: number, relY: number): [number, number] {
    if (!screenshotImgEl) return [0, 0];
    const rect = screenshotImgEl.getBoundingClientRect();
    return [relX * rect.width, relY * rect.height];
  }

  function drawShapeOnCanvas(ctx: CanvasRenderingContext2D, stroke: StrokeData, endX: number, endY: number) {
    ctx.save();
    const ox = stroke.offsetX || 0;
    const oy = stroke.offsetY || 0;
    const sx = stroke.scaleX ?? 1;
    const sy = stroke.scaleY ?? 1;

    ctx.strokeStyle = stroke.color;
    ctx.lineWidth = stroke.thickness;
    ctx.lineCap = 'round';
    ctx.lineJoin = 'round';
    ctx.globalCompositeOperation = 'source-over';

    const startX = stroke.startX * sx + ox;
    const startY = stroke.startY * sy + oy;
    const eX = endX * sx + ox;
    const eY = endY * sy + oy;

    if (stroke.tool === 'line') {
      ctx.beginPath();
      ctx.moveTo(startX, startY);
      ctx.lineTo(eX, eY);
      ctx.stroke();
    } else if (stroke.tool === 'arrow') {
      ctx.beginPath();
      ctx.moveTo(startX, startY);
      ctx.lineTo(eX, eY);
      ctx.stroke();
      const angle = Math.atan2(eY - startY, eX - startX);
      const headLen = Math.max(12, stroke.thickness * 4);
      ctx.beginPath();
      ctx.moveTo(eX, eY);
      ctx.lineTo(eX - headLen * Math.cos(angle - 0.4), eY - headLen * Math.sin(angle - 0.4));
      ctx.moveTo(eX, eY);
      ctx.lineTo(eX - headLen * Math.cos(angle + 0.4), eY - headLen * Math.sin(angle + 0.4));
      ctx.stroke();
    } else if (stroke.tool === 'rect') {
      ctx.beginPath();
      ctx.strokeRect(startX, startY, eX - startX, eY - startY);
    } else if (stroke.tool === 'circle') {
      const r = Math.sqrt(Math.pow(eX - startX, 2) + Math.pow(eY - startY, 2));
      ctx.beginPath();
      ctx.arc(startX, startY, r, 0, Math.PI * 2);
      ctx.stroke();
    } else if (stroke.tool === 'oval') {
      const rx = Math.abs(eX - startX) / 2;
      const ry = Math.abs(eY - startY) / 2;
      const cx = startX + (eX - startX) / 2;
      const cy = startY + (eY - startY) / 2;
      ctx.beginPath();
      ctx.ellipse(cx, cy, Math.max(1, rx), Math.max(1, ry), 0, 0, Math.PI * 2);
      ctx.stroke();
    } else if (stroke.tool === 'checkmark') {
      const w = eX - startX;
      const h = eY - startY;
      ctx.beginPath();
      ctx.moveTo(startX, startY + h * 0.5);
      ctx.lineTo(startX + w * 0.35, startY + h);
      ctx.lineTo(startX + w, startY);
      ctx.stroke();
    } else if (stroke.tool === 'cross') {
      ctx.beginPath();
      ctx.moveTo(startX, startY);
      ctx.lineTo(eX, eY);
      ctx.moveTo(eX, startY);
      ctx.lineTo(startX, eY);
      ctx.stroke();
    }
    ctx.restore();
  }

  function redrawLocalCanvas() {
    if (!drawCtx || !drawCanvasEl || !screenshotImgEl) return;
    const rect = screenshotImgEl.getBoundingClientRect();
    drawCtx.clearRect(0, 0, rect.width, rect.height);

    for (let i = 0; i < localStrokes.length; i++) {
      const s = localStrokes[i];
      const ox = s.offsetX || 0;
      const oy = s.offsetY || 0;
      const sx = s.scaleX ?? 1;
      const sy = s.scaleY ?? 1;

      if (s.tool === 'eraser') {
        drawCtx.globalCompositeOperation = 'destination-out';
        drawCtx.strokeStyle = 'rgba(0,0,0,1)';
        drawCtx.lineWidth = s.thickness;
        drawCtx.lineCap = 'round';
        drawCtx.lineJoin = 'round';
        if (s.points.length < 2) continue;
        drawCtx.beginPath();
        drawCtx.moveTo(s.points[0].x + ox, s.points[0].y + oy);
        for (let j = 1; j < s.points.length; j++) {
          drawCtx.lineTo(s.points[j].x + ox, s.points[j].y + oy);
        }
        drawCtx.stroke();
        drawCtx.globalCompositeOperation = 'source-over';
      } else if (s.tool === 'brush') {
        drawCtx.globalCompositeOperation = 'source-over';
        drawCtx.strokeStyle = s.color;
        drawCtx.lineWidth = s.thickness;
        drawCtx.lineCap = 'round';
        drawCtx.lineJoin = 'round';
        if (s.points.length < 2) continue;
        drawCtx.beginPath();
        drawCtx.moveTo(s.points[0].x + ox, s.points[0].y + oy);
        for (let j = 1; j < s.points.length; j++) {
          drawCtx.lineTo(s.points[j].x + ox, s.points[j].y + oy);
        }
        drawCtx.stroke();
      } else if (s.endX !== undefined && s.endY !== undefined) {
        drawShapeOnCanvas(drawCtx, s, s.endX, s.endY);
      }

      // Draw selection handles (Phase 3)
      if (i === selectedShapeIndex && activeTool === 'select') {
        drawSelectionHandles(drawCtx, s);
      }
    }
  }

  // === Phase 3: Selection & manipulation ===
  function getShapeBounds(s: StrokeData): {x: number, y: number, w: number, h: number} {
    const ox = s.offsetX || 0;
    const oy = s.offsetY || 0;
    const sx = s.scaleX ?? 1;
    const sy = s.scaleY ?? 1;

    if (s.tool === 'brush' || s.tool === 'eraser') {
      if (s.points.length === 0) return {x: 0, y: 0, w: 0, h: 0};
      let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
      for (const p of s.points) {
        minX = Math.min(minX, p.x + ox);
        minY = Math.min(minY, p.y + oy);
        maxX = Math.max(maxX, p.x + ox);
        maxY = Math.max(maxY, p.y + oy);
      }
      return {x: minX, y: minY, w: maxX - minX, h: maxY - minY};
    } else if (s.tool === 'circle') {
      const sX = s.startX * sx + ox;
      const sY = s.startY * sy + oy;
      const eX = (s.endX ?? s.startX) * sx + ox;
      const eY = (s.endY ?? s.startY) * sy + oy;
      const r = Math.sqrt(Math.pow(eX - sX, 2) + Math.pow(eY - sY, 2));
      return {x: sX - r, y: sY - r, w: r * 2, h: r * 2};
    } else {
      const sX = s.startX * sx + ox;
      const sY = s.startY * sy + oy;
      const eX = (s.endX ?? s.startX) * sx + ox;
      const eY = (s.endY ?? s.startY) * sy + oy;
      const x = Math.min(sX, eX);
      const y = Math.min(sY, eY);
      return {x, y, w: Math.abs(eX - sX), h: Math.abs(eY - sY)};
    }
  }

  function drawSelectionHandles(ctx: CanvasRenderingContext2D, s: StrokeData) {
    const b = getShapeBounds(s);
    const pad = 4;
    ctx.save();
    ctx.strokeStyle = '#0088FF';
    ctx.lineWidth = 1.5;
    ctx.setLineDash([4, 4]);
    ctx.strokeRect(b.x - pad, b.y - pad, b.w + pad * 2, b.h + pad * 2);
    ctx.setLineDash([]);

    // Corner handles
    const handles = [
      {x: b.x - pad, y: b.y - pad, cursor: 'nw'},
      {x: b.x + b.w + pad, y: b.y - pad, cursor: 'ne'},
      {x: b.x - pad, y: b.y + b.h + pad, cursor: 'sw'},
      {x: b.x + b.w + pad, y: b.y + b.h + pad, cursor: 'se'},
    ];
    for (const h of handles) {
      ctx.fillStyle = '#FFFFFF';
      ctx.strokeStyle = '#0088FF';
      ctx.lineWidth = 1.5;
      ctx.fillRect(h.x - HANDLE_SIZE/2, h.y - HANDLE_SIZE/2, HANDLE_SIZE, HANDLE_SIZE);
      ctx.strokeRect(h.x - HANDLE_SIZE/2, h.y - HANDLE_SIZE/2, HANDLE_SIZE, HANDLE_SIZE);
    }
    ctx.restore();
  }

  function hitTestHandle(s: StrokeData, cx: number, cy: number): string {
    const b = getShapeBounds(s);
    const pad = 4;
    const hs = HANDLE_SIZE;
    const handles = [
      {x: b.x - pad, y: b.y - pad, name: 'nw'},
      {x: b.x + b.w + pad, y: b.y - pad, name: 'ne'},
      {x: b.x - pad, y: b.y + b.h + pad, name: 'sw'},
      {x: b.x + b.w + pad, y: b.y + b.h + pad, name: 'se'},
    ];
    for (const h of handles) {
      if (cx >= h.x - hs && cx <= h.x + hs && cy >= h.y - hs && cy <= h.y + hs) {
        return h.name;
      }
    }
    return '';
  }

  function hitTestShape(cx: number, cy: number): number {
    // Iterate in reverse to select top shape first
    for (let i = localStrokes.length - 1; i >= 0; i--) {
      const b = getShapeBounds(localStrokes[i]);
      const pad = 6;
      if (cx >= b.x - pad && cx <= b.x + b.w + pad && cy >= b.y - pad && cy <= b.y + b.h + pad) {
        return i;
      }
    }
    return -1;
  }

  // === Event handlers ===
  function handleScreenshotMouseMove(e: MouseEvent) {
    const coords = getRelativeCoords(e);
    if (!coords) return;

    const now = Date.now();
    if (now - lastMoveTime < MOVE_THROTTLE_MS) return;
    lastMoveTime = now;

    if (activeTool === 'cursor') {
      localCursorPos = {x: coords.x, y: coords.y};
      SendCursorMove(coords.x, coords.y);
    } else if (activeTool === 'select') {
      const canvasCoords = getCanvasCoords(e);
      if (!canvasCoords) return;

      if (isDraggingShape && selectedShapeIndex >= 0 && dragOrigShape) {
        const dx = canvasCoords.x - dragStartX;
        const dy = canvasCoords.y - dragStartY;
        localStrokes[selectedShapeIndex].offsetX = (dragOrigShape.offsetX || 0) + dx;
        localStrokes[selectedShapeIndex].offsetY = (dragOrigShape.offsetY || 0) + dy;
        redrawLocalCanvas();
      } else if (isResizingShape && selectedShapeIndex >= 0 && dragOrigShape) {
        const dx = canvasCoords.x - dragStartX;
        const dy = canvasCoords.y - dragStartY;
        const origBounds = getShapeBounds(dragOrigShape);
        const origW = origBounds.w || 1;
        const origH = origBounds.h || 1;
        let newSx = dragOrigShape.scaleX ?? 1;
        let newSy = dragOrigShape.scaleY ?? 1;

        if (resizeHandle.includes('e')) {
          newSx = (dragOrigShape.scaleX ?? 1) * (1 + dx / origW);
        } else if (resizeHandle.includes('w')) {
          newSx = (dragOrigShape.scaleX ?? 1) * (1 - dx / origW);
          localStrokes[selectedShapeIndex].offsetX = (dragOrigShape.offsetX || 0) + dx;
        }
        if (resizeHandle.includes('s')) {
          newSy = (dragOrigShape.scaleY ?? 1) * (1 + dy / origH);
        } else if (resizeHandle.includes('n')) {
          newSy = (dragOrigShape.scaleY ?? 1) * (1 - dy / origH);
          localStrokes[selectedShapeIndex].offsetY = (dragOrigShape.offsetY || 0) + dy;
        }

        localStrokes[selectedShapeIndex].scaleX = Math.max(0.1, newSx);
        localStrokes[selectedShapeIndex].scaleY = Math.max(0.1, newSy);
        redrawLocalCanvas();
      }
      return;
    } else if (isDrawing && isDrawTool(activeTool)) {
      SendDrawMove(coords.x, coords.y);
      // Local drawing
      if (currentLocalStroke) {
        const [px, py] = toCanvasPixel(coords.x, coords.y);
        if (currentLocalStroke.tool === 'brush' || currentLocalStroke.tool === 'eraser') {
          currentLocalStroke.points.push({x: px, y: py});
          // Draw incremental line
          if (drawCtx && currentLocalStroke.points.length >= 2) {
            const prev = currentLocalStroke.points[currentLocalStroke.points.length - 2];
            if (currentLocalStroke.tool === 'eraser') {
              drawCtx.globalCompositeOperation = 'destination-out';
              drawCtx.strokeStyle = 'rgba(0,0,0,1)';
            } else {
              drawCtx.globalCompositeOperation = 'source-over';
              drawCtx.strokeStyle = currentLocalStroke.color;
            }
            drawCtx.lineWidth = currentLocalStroke.thickness;
            drawCtx.lineCap = 'round';
            drawCtx.lineJoin = 'round';
            drawCtx.beginPath();
            drawCtx.moveTo(prev.x, prev.y);
            drawCtx.lineTo(px, py);
            drawCtx.stroke();
            drawCtx.globalCompositeOperation = 'source-over';
          }
        } else {
          // Shape preview
          redrawLocalCanvas();
          if (drawCtx) {
            drawShapeOnCanvas(drawCtx, currentLocalStroke, px, py);
          }
        }
      }
    }
  }

  function handleScreenshotMouseDown(e: MouseEvent) {
    const coords = getRelativeCoords(e);
    if (!coords) return;

    if (activeTool === 'select') {
      const canvasCoords = getCanvasCoords(e);
      if (!canvasCoords) return;

      // Check handle first
      if (selectedShapeIndex >= 0) {
        const handle = hitTestHandle(localStrokes[selectedShapeIndex], canvasCoords.x, canvasCoords.y);
        if (handle) {
          isResizingShape = true;
          resizeHandle = handle;
          dragStartX = canvasCoords.x;
          dragStartY = canvasCoords.y;
          dragOrigShape = {...localStrokes[selectedShapeIndex]};
          return;
        }
      }

      // Check shape hit
      const hitIdx = hitTestShape(canvasCoords.x, canvasCoords.y);
      if (hitIdx >= 0) {
        selectedShapeIndex = hitIdx;
        isDraggingShape = true;
        dragStartX = canvasCoords.x;
        dragStartY = canvasCoords.y;
        dragOrigShape = {...localStrokes[hitIdx]};
        redrawLocalCanvas();
        return;
      }

      selectedShapeIndex = -1;
      redrawLocalCanvas();
      return;
    }

    if (activeTool === 'cursor') {
      localCursorPos = {x: coords.x, y: coords.y};
      SendCursorClick(coords.x, coords.y);
      // Add ripple effect
      const rippleId = ++rippleIdCounter;
      cursorRipples = [...cursorRipples, {id: rippleId, x: coords.x, y: coords.y}];
      setTimeout(() => {
        cursorRipples = cursorRipples.filter(r => r.id !== rippleId);
      }, 600);
    } else if (activeTool === 'hint') {
      if (!screenshotImgEl) return;
      const rect = screenshotImgEl.getBoundingClientRect();
      hintInputX = e.clientX - rect.left;
      hintInputY = e.clientY - rect.top;
      hintRelX = coords.x;
      hintRelY = coords.y;
      hintText = '';
      hintInputVisible = true;
      setTimeout(() => { if (hintInputEl) hintInputEl.focus(); }, 50);
    } else if (activeTool === 'text') {
      if (!screenshotImgEl) return;
      const rect = screenshotImgEl.getBoundingClientRect();
      textInputX = e.clientX - rect.left;
      textInputY = e.clientY - rect.top;
      textRelX = coords.x;
      textRelY = coords.y;
      textInputValue = '';
      textInputVisible = true;
      setTimeout(() => { if (textInputEl) textInputEl.focus(); }, 50);
    } else if (isDrawTool(activeTool)) {
      isDrawing = true;
      drawStartCoords = coords;
      const toolForSend = activeTool as string;
      SendDrawStart(coords.x, coords.y, drawColor, drawThickness, toolForSend);

      // Local drawing
      const [px, py] = toCanvasPixel(coords.x, coords.y);
      const lineW = Math.max(1, drawThickness * (screenshotImgEl?.getBoundingClientRect().width || 800));
      currentLocalStroke = {
        tool: toolForSend,
        color: drawColor,
        thickness: lineW,
        points: [{x: px, y: py}],
        startX: px,
        startY: py,
      };
    }
  }

  function handleScreenshotMouseUp(e: MouseEvent) {
    if (activeTool === 'select') {
      isDraggingShape = false;
      isResizingShape = false;
      dragOrigShape = null;
      return;
    }
    if (!isDrawing) return;
    const coords = getRelativeCoords(e);
    isDrawing = false;
    if (coords) {
      SendDrawEnd(coords.x, coords.y);
      // Finalize local stroke
      if (currentLocalStroke) {
        const [px, py] = toCanvasPixel(coords.x, coords.y);
        currentLocalStroke.endX = px;
        currentLocalStroke.endY = py;
        localStrokes = [...localStrokes, currentLocalStroke];
        currentLocalStroke = null;
        redrawLocalCanvas();
      }
    }
    drawStartCoords = null;
  }

  function localDrawUndo() {
    SendDrawUndo();
    if (localStrokes.length > 0) {
      localStrokes = localStrokes.slice(0, -1);
      selectedShapeIndex = -1;
      redrawLocalCanvas();
    }
  }

  function localDrawClear() {
    SendDrawClear();
    localStrokes = [];
    currentLocalStroke = null;
    selectedShapeIndex = -1;
    redrawLocalCanvas();
  }

  function deleteSelectedShape() {
    if (selectedShapeIndex >= 0 && selectedShapeIndex < localStrokes.length) {
      localStrokes = localStrokes.filter((_, i) => i !== selectedShapeIndex);
      selectedShapeIndex = -1;
      redrawLocalCanvas();
      // Also undo on remote (best effort — corresponds to the last stroke)
      SendDrawUndo();
    }
  }

  function handleHintSubmit() {
    if (hintText.trim()) {
      SendHint(hintRelX, hintRelY, hintText.trim(), 0);
    }
    hintInputVisible = false;
    hintText = '';
  }

  function handleHintKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleHintSubmit();
    } else if (e.key === 'Escape') {
      hintInputVisible = false;
      hintText = '';
    }
  }

  function handleTextSubmit() {
    if (textInputValue.trim()) {
      SendTextOverlay(textRelX, textRelY, textInputValue.trim(), drawColor);
    }
    textInputVisible = false;
    textInputValue = '';
  }

  function handleTextKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleTextSubmit();
    } else if (e.key === 'Escape') {
      textInputVisible = false;
      textInputValue = '';
    }
  }

  function handleGlobalMouseUp() {
    if (isDrawing) {
      isDrawing = false;
      if (currentLocalStroke) {
        localStrokes = [...localStrokes, currentLocalStroke];
        currentLocalStroke = null;
        redrawLocalCanvas();
      }
    }
    isDraggingShape = false;
    isResizingShape = false;
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Delete' || e.key === 'Backspace') {
      if (activeTool === 'select' && selectedShapeIndex >= 0 && !hintInputVisible && !textInputVisible) {
        e.preventDefault();
        deleteSelectedShape();
      }
    }
    if (e.key === 'Escape') {
      if (viewMode === 'fullscreen') {
        exitFullscreen();
      } else if (viewMode === 'editor') {
        closeEditor();
      }
    }
    // Ctrl+Z for undo
    if ((e.ctrlKey || e.metaKey) && e.key === 'z' && !hintInputVisible && !textInputVisible) {
      e.preventDefault();
      localDrawUndo();
    }
  }

  function getToolLabel(tool: ToolMode): string {
    const labels: Record<string, string> = {
      none: 'Нет',
      select: 'Выбор',
      cursor: 'Курсор',
      hint: 'Подсказка',
      text: 'Текст',
      brush: 'Кисть',
      eraser: 'Ластик',
      arrow: 'Стрелка',
      rect: 'Прямоугольник',
      circle: 'Круг',
      oval: 'Овал',
      line: 'Линия',
      checkmark: 'Галочка',
      cross: 'Крестик',
    };
    return labels[tool] || tool;
  }

  let resizeObserver: ResizeObserver | null = null;

  onMount(() => {
    window.addEventListener('mouseup', handleGlobalMouseUp);
    window.addEventListener('keydown', handleKeydown);
    resizeObserver = new ResizeObserver(() => {
      syncCanvasSize();
    });
  });

  onDestroy(() => {
    window.removeEventListener('mouseup', handleGlobalMouseUp);
    window.removeEventListener('keydown', handleKeydown);
    if (resizeObserver) resizeObserver.disconnect();
    if (activeTool === 'cursor') {
      SendCursorHide();
    }
  });

  // Watch for image element changes to attach observer
  $: if (screenshotImgEl && resizeObserver) {
    resizeObserver.disconnect();
    resizeObserver.observe(screenshotImgEl);
    requestAnimationFrame(syncCanvasSize);
  }
</script>

{#if (viewMode === 'editor' || viewMode === 'fullscreen') && editorScreenshot}
  <!-- ============ EDITOR / FULLSCREEN MODE ============ -->
  <div class="editor-container" class:fullscreen={viewMode === 'fullscreen'}>
    <!-- Side panel -->
    <aside class="editor-sidebar">
      <button class="back-btn" on:click={viewMode === 'fullscreen' ? exitFullscreen : closeEditor}>
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <polyline points="15 18 9 12 15 6"/>
        </svg>
        <span>{viewMode === 'fullscreen' ? 'Свернуть' : 'Назад'}</span>
      </button>

      {#if viewMode === 'editor'}
        <button class="fullscreen-btn" on:click={() => openFullscreen(editorScreenshot)}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="15 3 21 3 21 9"/>
            <polyline points="9 21 3 21 3 15"/>
            <line x1="21" y1="3" x2="14" y2="10"/>
            <line x1="3" y1="21" x2="10" y2="14"/>
          </svg>
          <span>На весь экран</span>
        </button>
      {/if}

      <div class="sidebar-section">
        <div class="section-title">Выбор</div>
        <div class="tool-grid">
          <button class="sidebar-tool-btn" class:active={activeTool === 'select'} on:click={() => setTool('select')} title="Выбрать фигуру">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M5 3l14 9-6 1-3 6-5-16z"/>
            </svg>
            <span class="tool-label">Выбор</span>
          </button>
        </div>
      </div>

      <div class="sidebar-section">
        <div class="section-title">Указатели</div>
        <div class="tool-grid">
          <button class="sidebar-tool-btn" class:active={activeTool === 'cursor'} on:click={() => setTool('cursor')} title="Призрачный курсор">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M3 3l7.07 16.97 2.51-7.39 7.39-2.51L3 3z"/>
            </svg>
            <span class="tool-label">Курсор</span>
          </button>
          <button class="sidebar-tool-btn" class:active={activeTool === 'hint'} on:click={() => setTool('hint')} title="Подсказка">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
            </svg>
            <span class="tool-label">Подсказка</span>
          </button>
          <button class="sidebar-tool-btn" class:active={activeTool === 'text'} on:click={() => setTool('text')} title="Текстовая метка">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="4 7 4 4 20 4 20 7"/>
              <line x1="9" y1="20" x2="15" y2="20"/>
              <line x1="12" y1="4" x2="12" y2="20"/>
            </svg>
            <span class="tool-label">Текст</span>
          </button>
        </div>
        {#if activeTool === 'hint'}
          <button class="sidebar-action-btn" on:click={() => ClearHints()}>Очистить подсказки</button>
        {/if}
        {#if activeTool === 'text'}
          <div class="sidebar-section-inline">
            <div class="section-title">Цвет текста</div>
            <div class="color-grid">
              {#each COLORS as color}
                <button
                  class="color-btn"
                  class:active={drawColor === color}
                  style="background: {color};"
                  on:click={() => drawColor = color}
                ></button>
              {/each}
            </div>
          </div>
          <button class="sidebar-action-btn" on:click={() => ClearTextOverlays()}>Очистить тексты</button>
        {/if}
      </div>

      <div class="sidebar-section">
        <div class="section-title">Рисование</div>
        <div class="tool-grid">
          <button class="sidebar-tool-btn" class:active={activeTool === 'brush'} on:click={() => setTool('brush')} title="Кисть">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M12 19l7-7 3 3-7 7-3-3z"/>
              <path d="M18 13l-1.5-7.5L2 2l3.5 14.5L13 18l5-5z"/>
            </svg>
            <span class="tool-label">Кисть</span>
          </button>
          <button class="sidebar-tool-btn" class:active={activeTool === 'eraser'} on:click={() => setTool('eraser')} title="Ластик">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M20 20H7L3 16l9-9 8 8-4 4z"/>
              <path d="M6.5 13.5l5-5"/>
            </svg>
            <span class="tool-label">Ластик</span>
          </button>
        </div>
      </div>

      <div class="sidebar-section">
        <div class="section-title">Фигуры</div>
        <div class="tool-grid">
          <button class="sidebar-tool-btn" class:active={activeTool === 'arrow'} on:click={() => setTool('arrow')} title="Стрелка">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="5" y1="12" x2="19" y2="12"/>
              <polyline points="12 5 19 12 12 19"/>
            </svg>
            <span class="tool-label">Стрелка</span>
          </button>
          <button class="sidebar-tool-btn" class:active={activeTool === 'line'} on:click={() => setTool('line')} title="Линия">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="4" y1="20" x2="20" y2="4"/>
            </svg>
            <span class="tool-label">Линия</span>
          </button>
          <button class="sidebar-tool-btn" class:active={activeTool === 'rect'} on:click={() => setTool('rect')} title="Прямоугольник">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
            </svg>
            <span class="tool-label">Прямоуг.</span>
          </button>
          <button class="sidebar-tool-btn" class:active={activeTool === 'circle'} on:click={() => setTool('circle')} title="Круг">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10"/>
            </svg>
            <span class="tool-label">Круг</span>
          </button>
          <button class="sidebar-tool-btn" class:active={activeTool === 'oval'} on:click={() => setTool('oval')} title="Овал">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <ellipse cx="12" cy="12" rx="10" ry="6"/>
            </svg>
            <span class="tool-label">Овал</span>
          </button>
          <button class="sidebar-tool-btn" class:active={activeTool === 'checkmark'} on:click={() => setTool('checkmark')} title="Галочка">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
              <polyline points="4 12 9 17 20 6"/>
            </svg>
            <span class="tool-label">Галочка</span>
          </button>
          <button class="sidebar-tool-btn" class:active={activeTool === 'cross'} on:click={() => setTool('cross')} title="Крестик">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
              <line x1="5" y1="5" x2="19" y2="19"/>
              <line x1="19" y1="5" x2="5" y2="19"/>
            </svg>
            <span class="tool-label">Крестик</span>
          </button>
        </div>
      </div>

      {#if isDrawTool(activeTool) || activeTool === 'select'}
        <div class="sidebar-section">
          {#if isDrawTool(activeTool)}
            <div class="section-title">Цвет</div>
            <div class="color-grid">
              {#each COLORS as color}
                <button
                  class="color-btn"
                  class:active={drawColor === color}
                  style="background: {color};"
                  on:click={() => drawColor = color}
                ></button>
              {/each}
            </div>

            <div class="section-title" style="margin-top: 12px;">Толщина</div>
            <div class="thickness-slider-container">
              <input
                type="range"
                min="0.001"
                max="0.01"
                step="0.0005"
                bind:value={drawThickness}
                class="thickness-slider"
              />
              <div class="thickness-preview-line" style="height: {Math.max(1, drawThickness * 300)}px;"></div>
            </div>
          {/if}

          <div class="section-title" style="margin-top: 8px;">Действия</div>
          <button class="sidebar-action-btn" on:click={localDrawUndo}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="1 4 1 10 7 10"/>
              <path d="M3.51 15a9 9 0 1 0 2.13-9.36L1 10"/>
            </svg>
            Отменить
          </button>
          {#if activeTool === 'select' && selectedShapeIndex >= 0}
            <button class="sidebar-action-btn danger" on:click={deleteSelectedShape}>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="3 6 5 6 21 6"/>
                <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
              </svg>
              Удалить фигуру
            </button>
          {/if}
          <button class="sidebar-action-btn danger" on:click={localDrawClear}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="3 6 5 6 21 6"/>
              <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
            </svg>
            Очистить всё
          </button>
        </div>
      {/if}
    </aside>

    <!-- Editor main area -->
    <main class="editor-main">
      <!-- Status bar top -->
      <div class="editor-top-bar">
        <div class="editor-screenshot-info">
          {#if editorScreenshot?.width}
            <span class="info-badge">{editorScreenshot.width} x {editorScreenshot.height}</span>
          {/if}
          {#if editorScreenshot?.timestamp}
            <span class="info-badge">{formatTime(editorScreenshot.timestamp)}</span>
          {/if}
          {#if localStrokes.length > 0}
            <span class="info-badge">{localStrokes.length} фигур</span>
          {/if}
        </div>
        <div class="editor-active-tool">
          {#if activeTool !== 'none'}
            <span class="active-tool-badge">{getToolLabel(activeTool)}</span>
          {:else}
            <span class="active-tool-badge inactive">Выберите инструмент</span>
          {/if}
        </div>
      </div>

      <!-- Screenshot canvas -->
      <div class="editor-canvas-area"
        class:cursor-crosshair={activeTool === 'hint' || activeTool === 'text' || isDrawTool(activeTool)}
        class:cursor-pointer={activeTool === 'cursor'}
        class:cursor-select={activeTool === 'select'}
      >
        <div class="editor-screenshot-wrapper">
          <img
            bind:this={screenshotImgEl}
            src={editorScreenshot.data}
            alt="Screenshot"
            class="editor-screenshot-image"
            draggable="false"
          />
          <canvas
            bind:this={drawCanvasEl}
            class="draw-overlay-canvas"
            on:mousemove={handleScreenshotMouseMove}
            on:mousedown={handleScreenshotMouseDown}
            on:mouseup={handleScreenshotMouseUp}
          ></canvas>
          <!-- Local ghost cursor for controller -->
          {#if activeTool === 'cursor' && localCursorPos}
            <svg
              class="local-ghost-cursor"
              style="left: {localCursorPos.x * 100}%; top: {localCursorPos.y * 100}%;"
              viewBox="0 0 24 24" fill="#f97316" stroke="#fff" stroke-width="1"
            >
              <path d="M3 3l7.07 16.97 2.51-7.39 7.39-2.51L3 3z"/>
            </svg>
          {/if}
          {#each cursorRipples as ripple (ripple.id)}
            <div
              class="local-cursor-ripple"
              style="left: {ripple.x * 100}%; top: {ripple.y * 100}%;"
            ></div>
          {/each}
          {#if hintInputVisible}
            <div class="hint-input-container" style="left: {hintInputX}px; top: {hintInputY}px;">
              <input
                bind:this={hintInputEl}
                bind:value={hintText}
                on:keydown={handleHintKeydown}
                on:blur={() => { hintInputVisible = false; }}
                class="hint-input"
                placeholder="Введите подсказку..."
                maxlength="200"
              />
            </div>
          {/if}
          {#if textInputVisible}
            <div class="hint-input-container" style="left: {textInputX}px; top: {textInputY}px;">
              <input
                bind:this={textInputEl}
                bind:value={textInputValue}
                on:keydown={handleTextKeydown}
                on:blur={() => { textInputVisible = false; }}
                class="hint-input"
                placeholder="Введите текст..."
                maxlength="200"
                style="border-color: {drawColor};"
              />
            </div>
          {/if}
        </div>
      </div>
    </main>
  </div>

{:else}
  <!-- ============ GALLERY MODE ============ -->
  <div class="screenshot-container">
    {#if role === 'controller' && isConnected && history.length > 0}
      <aside class="history-sidebar">
        <div class="history-header">
          <span class="history-title">История ({history.length})</span>
          <button class="btn-icon-sm" on:click={clearHistory} title="Очистить историю">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="3 6 5 6 21 6"/>
              <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
            </svg>
          </button>
        </div>
        <div class="history-list">
          {#each [...history].reverse() as entry (entry.id)}
            <button
              class="history-item {selectedId === entry.id ? 'active' : ''}"
              on:click={() => selectScreenshot(entry.id)}
            >
              {#if entry.data}
                <img src={entry.data} alt="Screenshot {entry.id}" class="history-thumb" />
              {:else}
                <div class="history-thumb placeholder">
                  {#if loadingId === entry.id}
                    <div class="spinner-sm"></div>
                  {:else}
                    <span>#{entry.id}</span>
                  {/if}
                </div>
              {/if}
              <div class="history-meta">
                <span class="history-time">{formatTime(entry.timestamp)}</span>
                <span class="history-size">{entry.width}x{entry.height}</span>
              </div>
            </button>
          {/each}
        </div>
      </aside>
    {/if}

    <main class="screenshot-main">
      <div class="screenshot-content">
        <div class="panel-header">
          <div>
            <h1 class="panel-title">Скриншоты</h1>
            <p class="panel-subtitle">
              {role === 'controller'
                ? 'Просмотр экрана клиента в реальном времени'
                : 'Ваш экран транслируется контроллеру'}
            </p>
          </div>
          {#if screenshotData?.timestamp}
            <span class="timestamp">{formatTime(screenshotData.timestamp)}</span>
          {/if}
        </div>

        {#if !isConnected}
          <div class="empty-state card">
            <div class="empty-icon">
              <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
                <circle cx="8.5" cy="8.5" r="1.5"/>
                <polyline points="21 15 16 10 5 21"/>
              </svg>
            </div>
            <p class="empty-title">Сначала подключитесь к комнате</p>
          </div>
        {:else if role === 'client'}
          <div class="empty-state card">
            <div class="empty-icon streaming">
              <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/>
                <circle cx="12" cy="12" r="3"/>
              </svg>
            </div>
            <p class="empty-title">Ваш экран транслируется</p>
            <p class="empty-subtitle">Контроллер видит ваш экран в реальном времени</p>
          </div>
        {:else if screenshotData}
          <div class="screenshot-view card">
            <div class="screenshot-wrapper">
              <img
                src={screenshotData.data}
                alt="Screenshot"
                class="screenshot-image"
                draggable="false"
              />
            </div>
            <div class="screenshot-meta">
              <span>{screenshotData.width} x {screenshotData.height}</span>
              {#if selectedId}
                <span>#{selectedId}</span>
              {/if}
            </div>
          </div>
        {:else}
          <div class="empty-state card">
            <div class="empty-icon loading">
              <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                <circle cx="12" cy="12" r="10"/>
                <polyline points="12 6 12 12 16 14"/>
              </svg>
            </div>
            <p class="empty-title">Ожидание скриншота от клиента...</p>
            <p class="empty-subtitle">Скриншоты обновляются автоматически</p>
          </div>
        {/if}

        {#if role === 'controller'}
          <div class="controls">
            <div class="controls-row">
              <button
                class="btn {saveSuccess ? 'btn-success' : 'btn-secondary'} save-btn"
                disabled={!isConnected || !screenshotData || saving}
                on:click={saveScreenshot}
              >
                {#if saving}
                  <div class="spinner-sm"></div>
                  <span>Сохранение...</span>
                {:else if saveSuccess}
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <polyline points="20 6 9 17 4 12"/>
                  </svg>
                  <span>Сохранено!</span>
                {:else}
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/>
                    <polyline points="17 21 17 13 7 13 7 21"/>
                    <polyline points="7 3 7 8 15 8"/>
                  </svg>
                  <span>Сохранить</span>
                {/if}
              </button>
              <button
                class="btn btn-primary editor-open-btn"
                disabled={!isConnected || !screenshotData}
                on:click={() => openEditor(screenshotData)}
              >
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                  <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                </svg>
                <span>Работа со снимком</span>
              </button>
              <button
                class="btn btn-secondary fullscreen-gallery-btn"
                disabled={!isConnected || !screenshotData}
                on:click={() => openFullscreen(screenshotData)}
                title="На весь экран"
              >
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <polyline points="15 3 21 3 21 9"/>
                  <polyline points="9 21 3 21 3 15"/>
                  <line x1="21" y1="3" x2="14" y2="10"/>
                  <line x1="3" y1="21" x2="10" y2="14"/>
                </svg>
              </button>
            </div>
          </div>
        {/if}
      </div>
    </main>
  </div>
{/if}

<style>
  /* ============ GALLERY MODE STYLES ============ */
  .screenshot-container {
    height: 100%;
    display: flex;
  }

  .history-sidebar {
    width: 180px;
    background: var(--glass-bg);
    backdrop-filter: blur(var(--glass-blur));
    -webkit-backdrop-filter: blur(var(--glass-blur));
    border-right: 1px solid var(--border-primary);
    display: flex;
    flex-direction: column;
  }

  .history-header {
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--border-secondary);
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .history-title {
    font-size: var(--text-sm);
    font-weight: 600;
    color: var(--text-secondary);
  }

  .btn-icon-sm {
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    border-radius: var(--radius-md);
    color: var(--text-tertiary);
    cursor: pointer;
    transition: all var(--duration-fast) var(--ease-out);
  }

  .btn-icon-sm:hover {
    background: var(--bg-hover);
    color: var(--color-error);
  }

  .history-list {
    flex: 1;
    overflow-y: auto;
    padding: var(--space-2);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .history-item {
    width: 100%;
    background: var(--bg-tertiary);
    border: 2px solid var(--border-primary);
    border-radius: var(--radius-lg);
    overflow: hidden;
    cursor: pointer;
    transition: all var(--duration-fast) var(--ease-out);
    text-align: left;
    padding: 0;
  }

  .history-item:hover {
    border-color: var(--border-hover);
  }

  .history-item.active {
    border-color: var(--accent-primary);
    box-shadow: 0 0 0 2px var(--accent-primary-muted);
  }

  .history-thumb {
    width: 100%;
    height: 80px;
    object-fit: cover;
    background: var(--bg-tertiary);
    display: block;
  }

  .history-thumb.placeholder {
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .history-meta {
    padding: var(--space-2);
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .history-time {
    font-size: var(--text-xs);
    color: var(--text-secondary);
  }

  .history-size {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .screenshot-main {
    flex: 1;
    padding: var(--space-8);
    overflow: auto;
  }

  .screenshot-content {
    max-width: 1000px;
    margin: 0 auto;
  }

  .panel-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    margin-bottom: var(--space-6);
  }

  .panel-title {
    font-size: var(--text-2xl);
    font-weight: 700;
    color: var(--text-primary);
    margin: 0 0 var(--space-2) 0;
    letter-spacing: var(--tracking-tight);
  }

  .panel-subtitle {
    font-size: var(--text-base);
    color: var(--text-secondary);
    margin: 0;
  }

  .timestamp {
    font-size: var(--text-sm);
    color: var(--text-tertiary);
  }

  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: var(--space-16);
    text-align: center;
  }

  .empty-icon {
    width: 80px;
    height: 80px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-tertiary);
    border-radius: var(--radius-2xl);
    color: var(--text-muted);
    margin-bottom: var(--space-4);
  }

  .empty-icon.streaming {
    background: var(--color-success-muted);
    color: var(--color-success);
    animation: pulse-glow 2s var(--ease-in-out) infinite;
  }

  .empty-icon.loading {
    animation: pulse-glow 2s var(--ease-in-out) infinite;
  }

  .empty-title {
    font-size: var(--text-lg);
    font-weight: 500;
    color: var(--text-secondary);
    margin: 0 0 var(--space-2) 0;
  }

  .empty-subtitle {
    font-size: var(--text-sm);
    color: var(--text-tertiary);
    margin: 0;
  }

  .screenshot-view {
    padding: var(--space-2);
  }

  .screenshot-wrapper {
    position: relative;
  }

  .screenshot-image {
    width: 100%;
    border-radius: var(--radius-lg);
    display: block;
  }

  .screenshot-meta {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-3) var(--space-2) var(--space-1);
    font-size: var(--text-sm);
    color: var(--text-tertiary);
  }

  .controls {
    margin-top: var(--space-6);
  }

  .controls-row {
    display: flex;
    gap: var(--space-3);
  }

  .save-btn {
    flex: 1;
    padding: var(--space-4);
  }

  .editor-open-btn {
    flex: 1;
    padding: var(--space-4);
  }

  .fullscreen-gallery-btn {
    padding: var(--space-4);
    flex: 0 0 auto;
  }

  /* ============ EDITOR MODE STYLES ============ */
  .editor-container {
    height: 100%;
    display: flex;
  }

  .editor-container.fullscreen {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 9999;
    background: var(--bg-primary);
  }

  .editor-sidebar {
    width: 200px;
    background: var(--glass-bg);
    backdrop-filter: blur(var(--glass-blur));
    -webkit-backdrop-filter: blur(var(--glass-blur));
    border-right: 1px solid var(--border-primary);
    display: flex;
    flex-direction: column;
    padding: var(--space-3);
    gap: var(--space-2);
    overflow-y: auto;
  }

  .back-btn {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    background: transparent;
    border: 1px solid var(--border-secondary);
    border-radius: var(--radius-lg);
    color: var(--text-secondary);
    cursor: pointer;
    font-size: var(--text-sm);
    font-weight: 500;
    transition: all var(--duration-fast) var(--ease-out);
    margin-bottom: var(--space-2);
  }

  .back-btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
    border-color: var(--border-hover);
  }

  .fullscreen-btn {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    background: var(--accent-primary-muted);
    border: 1px solid var(--accent-primary);
    border-radius: var(--radius-lg);
    color: var(--accent-primary);
    cursor: pointer;
    font-size: var(--text-sm);
    font-weight: 500;
    transition: all var(--duration-fast) var(--ease-out);
    margin-bottom: var(--space-2);
  }

  .fullscreen-btn:hover {
    background: var(--accent-primary);
    color: #fff;
  }

  .sidebar-section {
    padding: var(--space-2) 0;
    border-bottom: 1px solid var(--border-secondary);
  }

  .sidebar-section:last-child {
    border-bottom: none;
  }

  .section-title {
    font-size: var(--text-xs);
    font-weight: 600;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    margin-bottom: var(--space-2);
    padding: 0 var(--space-1);
  }

  .tool-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-1);
  }

  .sidebar-tool-btn {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    padding: var(--space-2) var(--space-1);
    background: transparent;
    border: 2px solid transparent;
    border-radius: var(--radius-lg);
    color: var(--text-secondary);
    cursor: pointer;
    transition: all var(--duration-fast) var(--ease-out);
    font-size: 0;
  }

  .sidebar-tool-btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .sidebar-tool-btn.active {
    background: var(--accent-primary-muted);
    border-color: var(--accent-primary);
    color: var(--accent-primary);
  }

  .tool-label {
    font-size: 10px;
    line-height: 1.2;
    white-space: nowrap;
  }

  .color-grid {
    display: flex;
    gap: var(--space-1);
    flex-wrap: wrap;
  }

  .color-btn {
    width: 24px;
    height: 24px;
    border-radius: var(--radius-full);
    border: 2px solid transparent;
    cursor: pointer;
    transition: all var(--duration-fast) var(--ease-out);
  }

  .color-btn:hover {
    transform: scale(1.15);
  }

  .color-btn.active {
    border-color: var(--text-primary);
    box-shadow: 0 0 0 2px var(--bg-primary);
  }

  .thickness-slider-container {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-1) 0;
  }

  .thickness-slider {
    flex: 1;
    -webkit-appearance: none;
    appearance: none;
    height: 4px;
    background: var(--border-secondary);
    border-radius: 2px;
    outline: none;
  }

  .thickness-slider::-webkit-slider-thumb {
    -webkit-appearance: none;
    appearance: none;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background: var(--accent-primary);
    cursor: pointer;
    border: none;
  }

  .thickness-preview-line {
    width: 24px;
    min-height: 1px;
    background: var(--text-secondary);
    border-radius: 4px;
  }

  .sidebar-action-btn {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    background: var(--bg-tertiary);
    border: 1px solid var(--border-secondary);
    border-radius: var(--radius-md);
    color: var(--text-secondary);
    cursor: pointer;
    font-size: var(--text-xs);
    transition: all var(--duration-fast) var(--ease-out);
    margin-top: var(--space-1);
  }

  .sidebar-action-btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .sidebar-action-btn.danger:hover {
    background: rgba(239, 68, 68, 0.1);
    color: var(--color-error);
    border-color: var(--color-error);
  }

  .editor-main {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .editor-top-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--border-secondary);
    background: var(--glass-bg);
    backdrop-filter: blur(var(--glass-blur));
    -webkit-backdrop-filter: blur(var(--glass-blur));
  }

  .editor-screenshot-info {
    display: flex;
    gap: var(--space-2);
  }

  .info-badge {
    font-size: var(--text-xs);
    color: var(--text-tertiary);
    background: var(--bg-tertiary);
    padding: 2px 8px;
    border-radius: var(--radius-md);
  }

  .active-tool-badge {
    font-size: var(--text-sm);
    font-weight: 600;
    color: var(--accent-primary);
    background: var(--accent-primary-muted);
    padding: 4px 12px;
    border-radius: var(--radius-lg);
  }

  .active-tool-badge.inactive {
    color: var(--text-muted);
    background: var(--bg-tertiary);
  }

  .editor-canvas-area {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: auto;
    padding: var(--space-4);
    background: var(--bg-tertiary);
  }

  .editor-canvas-area.cursor-crosshair {
    cursor: crosshair;
  }

  .editor-canvas-area.cursor-pointer {
    cursor: none;
  }

  .editor-canvas-area.cursor-select {
    cursor: default;
  }

  .editor-screenshot-wrapper {
    position: relative;
    max-width: 100%;
    max-height: 100%;
  }

  .editor-screenshot-image {
    max-width: 100%;
    max-height: calc(100vh - 120px);
    border-radius: var(--radius-lg);
    display: block;
    box-shadow: 0 4px 24px rgba(0,0,0,0.3);
    user-select: none;
    -webkit-user-select: none;
  }

  .fullscreen .editor-screenshot-image {
    max-height: calc(100vh - 80px);
  }

  .draw-overlay-canvas {
    position: absolute;
    top: 0;
    left: 0;
    border-radius: var(--radius-lg);
    pointer-events: auto;
  }

  .editor-canvas-area.cursor-crosshair .draw-overlay-canvas {
    cursor: crosshair;
  }

  .editor-canvas-area.cursor-pointer .draw-overlay-canvas {
    cursor: none;
  }

  .editor-canvas-area.cursor-select .draw-overlay-canvas {
    cursor: default;
  }

  /* Local ghost cursor on controller */
  .local-ghost-cursor {
    position: absolute;
    width: 24px;
    height: 24px;
    pointer-events: none;
    z-index: 5;
    filter: drop-shadow(0 2px 4px rgba(0,0,0,0.4));
    transition: left 50ms ease-out, top 50ms ease-out;
    transform: translate(-2px, -2px);
  }

  .local-cursor-ripple {
    position: absolute;
    width: 40px;
    height: 40px;
    border-radius: 50%;
    border: 2px solid #f97316;
    pointer-events: none;
    z-index: 4;
    transform: translate(-50%, -50%) scale(0.5);
    animation: local-ripple 0.6s ease-out forwards;
  }

  @keyframes local-ripple {
    0%   { transform: translate(-50%, -50%) scale(0.5); opacity: 1; }
    100% { transform: translate(-50%, -50%) scale(2.5); opacity: 0; }
  }

  /* Hint input */
  .hint-input-container {
    position: absolute;
    z-index: 10;
    transform: translateY(-100%);
  }

  .hint-input {
    width: 220px;
    padding: 8px 12px;
    background: var(--bg-secondary);
    border: 2px solid var(--accent-primary);
    border-radius: var(--radius-lg);
    color: var(--text-primary);
    font-size: var(--text-sm);
    outline: none;
    box-shadow: 0 4px 16px rgba(0,0,0,0.3);
  }

  .hint-input::placeholder {
    color: var(--text-muted);
  }

  /* Spinner */
  .spinner-sm {
    width: 16px;
    height: 16px;
    border: 2px solid currentColor;
    border-top-color: transparent;
    border-radius: var(--radius-full);
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  @keyframes pulse-glow {
    0%, 100% {
      opacity: 1;
    }
    50% {
      opacity: 0.6;
    }
  }
</style>
