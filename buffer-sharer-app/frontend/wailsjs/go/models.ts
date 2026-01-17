export namespace main {
	
	export class ConnectionStatus {
	    connected: boolean;
	    roomCode: string;
	    role: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connected = source["connected"];
	        this.roomCode = source["roomCode"];
	        this.role = source["role"];
	        this.error = source["error"];
	    }
	}
	export class PermissionInfoJS {
	    type: string;
	    status: string;
	    name: string;
	    description: string;
	    required: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PermissionInfoJS(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.status = source["status"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.required = source["required"];
	    }
	}
	export class ScreenshotHistoryEntryJS {
	    id: number;
	    timestamp: string;
	    width: number;
	    height: number;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new ScreenshotHistoryEntryJS(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = source["timestamp"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.size = source["size"];
	    }
	}
	export class Settings {
	    middlewareHost: string;
	    middlewarePort: number;
	    screenshotInterval: number;
	    screenshotQuality: number;
	    clipboardSync: boolean;
	    hotkeyToggle: string;
	    hotkeyScreenshot: string;
	    hotkeyPaste: string;
	    hotkeyInvisibility: string;
	    autoConnect: boolean;
	    lastRole: string;
	    lastRoomCode: string;
	    soundEnabled: boolean;
	    theme: string;
	    screenshotSaveDir: string;
	    screenshotHistoryLimit: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.middlewareHost = source["middlewareHost"];
	        this.middlewarePort = source["middlewarePort"];
	        this.screenshotInterval = source["screenshotInterval"];
	        this.screenshotQuality = source["screenshotQuality"];
	        this.clipboardSync = source["clipboardSync"];
	        this.hotkeyToggle = source["hotkeyToggle"];
	        this.hotkeyScreenshot = source["hotkeyScreenshot"];
	        this.hotkeyPaste = source["hotkeyPaste"];
	        this.hotkeyInvisibility = source["hotkeyInvisibility"];
	        this.autoConnect = source["autoConnect"];
	        this.lastRole = source["lastRole"];
	        this.lastRoomCode = source["lastRoomCode"];
	        this.soundEnabled = source["soundEnabled"];
	        this.theme = source["theme"];
	        this.screenshotSaveDir = source["screenshotSaveDir"];
	        this.screenshotHistoryLimit = source["screenshotHistoryLimit"];
	    }
	}
	export class Statistics {
	    screenshotsSent: number;
	    screenshotsReceived: number;
	    textsSent: number;
	    textsReceived: number;
	    bytesSent: number;
	    bytesReceived: number;
	    // Go type: time
	    connectedAt: any;
	    totalConnectTime: number;
	
	    static createFrom(source: any = {}) {
	        return new Statistics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.screenshotsSent = source["screenshotsSent"];
	        this.screenshotsReceived = source["screenshotsReceived"];
	        this.textsSent = source["textsSent"];
	        this.textsReceived = source["textsReceived"];
	        this.bytesSent = source["bytesSent"];
	        this.bytesReceived = source["bytesReceived"];
	        this.connectedAt = this.convertValues(source["connectedAt"], null);
	        this.totalConnectTime = source["totalConnectTime"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TextHistoryEntry {
	    text: string;
	    direction: string;
	    // Go type: time
	    timestamp: any;
	
	    static createFrom(source: any = {}) {
	        return new TextHistoryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.direction = source["direction"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

