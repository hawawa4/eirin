export namespace app {
	
	export class AppInfo {
	    dbPath: string;
	    serverPort: number;
	    serverUrl: string;
	    portSource: string;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dbPath = source["dbPath"];
	        this.serverPort = source["serverPort"];
	        this.serverUrl = source["serverUrl"];
	        this.portSource = source["portSource"];
	    }
	}
	export class AtlasFrameSize {
	    width: number;
	    height: number;
	
	    static createFrom(source: any = {}) {
	        return new AtlasFrameSize(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.width = source["width"];
	        this.height = source["height"];
	    }
	}
	export class AtlasIndexEntry {
	    nasPath: string;
	    name: string;
	    object: string;
	    frameType: string;
	    ra: number;
	    dec: number;
	    pixelScale: number;
	    rotation: number;
	
	    static createFrom(source: any = {}) {
	        return new AtlasIndexEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nasPath = source["nasPath"];
	        this.name = source["name"];
	        this.object = source["object"];
	        this.frameType = source["frameType"];
	        this.ra = source["ra"];
	        this.dec = source["dec"];
	        this.pixelScale = source["pixelScale"];
	        this.rotation = source["rotation"];
	    }
	}
	export class CatalogObject {
	    ra: number;
	    dec: number;
	    name: string;
	    type: string;
	    mag: number;
	
	    static createFrom(source: any = {}) {
	        return new CatalogObject(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ra = source["ra"];
	        this.dec = source["dec"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.mag = source["mag"];
	    }
	}
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
	    isRejected: boolean;
	    rejectionReason: string;
	
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
	        this.isRejected = source["isRejected"];
	        this.rejectionReason = source["rejectionReason"];
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
	export class LibraryFrame {
	    nasPath: string;
	    fileName: string;
	    frameType: string;
	    object: string;
	    filter: string;
	    expTime: number;
	    dateObs: string;
	    gain: number;
	    ccdTemp: number;
	    telescope: string;
	    instrument: string;
	    fileSize: number;
	    isRejected: boolean;
	    ra: number;
	    dec: number;
	    pixelScale: number;
	    rotation: number;
	    wcsSolved: boolean;
	    fwhm: number;
	    fwhmUnit: string;
	    background: number;
	    noise: number;
	    snr: number;
	    starCount: number;
	    qualityAnalyzed: boolean;
	    moonPhase: number;
	
	    static createFrom(source: any = {}) {
	        return new LibraryFrame(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nasPath = source["nasPath"];
	        this.fileName = source["fileName"];
	        this.frameType = source["frameType"];
	        this.object = source["object"];
	        this.filter = source["filter"];
	        this.expTime = source["expTime"];
	        this.dateObs = source["dateObs"];
	        this.gain = source["gain"];
	        this.ccdTemp = source["ccdTemp"];
	        this.telescope = source["telescope"];
	        this.instrument = source["instrument"];
	        this.fileSize = source["fileSize"];
	        this.isRejected = source["isRejected"];
	        this.ra = source["ra"];
	        this.dec = source["dec"];
	        this.pixelScale = source["pixelScale"];
	        this.rotation = source["rotation"];
	        this.wcsSolved = source["wcsSolved"];
	        this.fwhm = source["fwhm"];
	        this.fwhmUnit = source["fwhmUnit"];
	        this.background = source["background"];
	        this.noise = source["noise"];
	        this.snr = source["snr"];
	        this.starCount = source["starCount"];
	        this.qualityAnalyzed = source["qualityAnalyzed"];
	        this.moonPhase = source["moonPhase"];
	    }
	}
	export class PagedLightFrames {
	    frames: LibraryFrame[];
	    hasMore: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PagedLightFrames(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.frames = this.convertValues(source["frames"], LibraryFrame);
	        this.hasMore = source["hasMore"];
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
	export class Project {
	    id: number;
	    name: string;
	    description: string;
	    folder: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Project(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.folder = source["folder"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class ProjectOutputFile {
	    name: string;
	    path: string;
	    size: number;
	    modTime: string;
	
	    static createFrom(source: any = {}) {
	        return new ProjectOutputFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.modTime = source["modTime"];
	    }
	}
	export class StorageNode {
	    label: string;
	    totalBytes: number;
	    frameCount: number;
	    children?: StorageNode[];
	
	    static createFrom(source: any = {}) {
	        return new StorageNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.totalBytes = source["totalBytes"];
	        this.frameCount = source["frameCount"];
	        this.children = this.convertValues(source["children"], StorageNode);
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
	export class SuggestResult {
	    frame: LibraryFrame;
	    groupMedian: number;
	    groupSigma: number;
	    sigmas: number;
	
	    static createFrom(source: any = {}) {
	        return new SuggestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.frame = this.convertValues(source["frame"], LibraryFrame);
	        this.groupMedian = source["groupMedian"];
	        this.groupSigma = source["groupSigma"];
	        this.sigmas = source["sigmas"];
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

export namespace catalog {
	
	export class Annotation {
	    x: number;
	    y: number;
	    label: string;
	    type: string;
	    mag: number;
	
	    static createFrom(source: any = {}) {
	        return new Annotation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	        this.label = source["label"];
	        this.type = source["type"];
	        this.mag = source["mag"];
	    }
	}

}

export namespace fits {
	
	export class ChannelStats {
	    median: number;
	    sigma: number;
	
	    static createFrom(source: any = {}) {
	        return new ChannelStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.median = source["median"];
	        this.sigma = source["sigma"];
	    }
	}
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
	    pixelScale: number;
	    rotation: number;
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
	        this.pixelScale = source["pixelScale"];
	        this.rotation = source["rotation"];
	        this.extra = source["extra"];
	    }
	}
	export class RawPreviewData {
	    data: string;
	    width: number;
	    height: number;
	    channels: number;
	    stats: ChannelStats[];
	
	    static createFrom(source: any = {}) {
	        return new RawPreviewData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.data = source["data"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.channels = source["channels"];
	        this.stats = this.convertValues(source["stats"], ChannelStats);
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

export namespace importer {
	
	export class Candidate {
	    sourcePath: string;
	    relativePath: string;
	    destPath: string;
	    fileSize: number;
	
	    static createFrom(source: any = {}) {
	        return new Candidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourcePath = source["sourcePath"];
	        this.relativePath = source["relativePath"];
	        this.destPath = source["destPath"];
	        this.fileSize = source["fileSize"];
	    }
	}

}

export namespace siril {
	
	export class SirilInfo {
	    executable: string;
	    version: string;
	    available: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SirilInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.executable = source["executable"];
	        this.version = source["version"];
	        this.available = source["available"];
	    }
	}

}

export namespace store {
	
	export class Prefs {
	    rootFolder: string;
	    basicCollapsed: boolean;
	    advancedCollapsed: boolean;
	    stretchEnabled: boolean;
	    stretchLevel: number;
	    columnConfig: string;
	    libraryColumnConfig: string;
	    sirilPath: string;
	    projectsFolder: string;
	    theme: string;
	
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
	        this.libraryColumnConfig = source["libraryColumnConfig"];
	        this.sirilPath = source["sirilPath"];
	        this.projectsFolder = source["projectsFolder"];
	        this.theme = source["theme"];
	    }
	}

}

