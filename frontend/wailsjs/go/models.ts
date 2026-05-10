export namespace browser {
	
	export class EnrichedFileEntry {
	    name: string;
	    path: string;
	    isDir: boolean;
	    // Go type: time
	    modTime: any;
	    size: number;
	    object: string;
	    filter: string;
	    expTime: number;
	    dateObs: string;
	    gain: number;
	    ccdTemp: number;
	    telescope: string;
	    instrument: string;
	    hasMeta: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EnrichedFileEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.isDir = source["isDir"];
	        this.modTime = this.convertValues(source["modTime"], null);
	        this.size = source["size"];
	        this.object = source["object"];
	        this.filter = source["filter"];
	        this.expTime = source["expTime"];
	        this.dateObs = source["dateObs"];
	        this.gain = source["gain"];
	        this.ccdTemp = source["ccdTemp"];
	        this.telescope = source["telescope"];
	        this.instrument = source["instrument"];
	        this.hasMeta = source["hasMeta"];
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
	export class FileEntry {
	    name: string;
	    path: string;
	    isDir: boolean;
	    // Go type: time
	    modTime: any;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new FileEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.isDir = source["isDir"];
	        this.modTime = this.convertValues(source["modTime"], null);
	        this.size = source["size"];
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

export namespace fits {
	
	export class FITSHeader {
	    width: number;
	    height: number;
	    channels: number;
	    bitpix: number;
	    object: string;
	    telescope: string;
	    instrument: string;
	    filter: string;
	    exptime: number;
	    dateObs: string;
	    gain: number;
	    offset: number;
	    ccdTemp: number;
	    ra: number;
	    dec: number;
	    xbinning: number;
	    ybinning: number;
	    focalLen: number;
	    siteElev: number;
	    siteLat: number;
	    siteLong: number;
	    extra: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new FITSHeader(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.width = source["width"];
	        this.height = source["height"];
	        this.channels = source["channels"];
	        this.bitpix = source["bitpix"];
	        this.object = source["object"];
	        this.telescope = source["telescope"];
	        this.instrument = source["instrument"];
	        this.filter = source["filter"];
	        this.exptime = source["exptime"];
	        this.dateObs = source["dateObs"];
	        this.gain = source["gain"];
	        this.offset = source["offset"];
	        this.ccdTemp = source["ccdTemp"];
	        this.ra = source["ra"];
	        this.dec = source["dec"];
	        this.xbinning = source["xbinning"];
	        this.ybinning = source["ybinning"];
	        this.focalLen = source["focalLen"];
	        this.siteElev = source["siteElev"];
	        this.siteLat = source["siteLat"];
	        this.siteLong = source["siteLong"];
	        this.extra = source["extra"];
	    }
	}

}

export namespace prefs {
	
	export class Prefs {
	    rootFolder: string;
	    basicCollapsed: boolean;
	    advancedCollapsed: boolean;
	    stretchEnabled: boolean;
	    stretchLevel: number;
	    columnConfig: string;
	
	    static createFrom(source: any = {}) {
	        return new Prefs(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rootFolder = source["rootFolder"];
	        this.basicCollapsed = source["basicCollapsed"];
	        this.advancedCollapsed = source["advancedCollapsed"];
	        this.stretchEnabled = source["stretchEnabled"];
	        this.stretchLevel = source["stretchLevel"];
	        this.columnConfig = source["columnConfig"];
	    }
	}

}

