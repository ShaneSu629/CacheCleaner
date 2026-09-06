export namespace main {
	
	export class CleanFailure {
	    path: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new CleanFailure(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.error = source["error"];
	    }
	}
	export class CleanResult {
	    freed: number;
	    count: number;
	    cleaned: string[];
	    failed: CleanFailure[];
	
	    static createFrom(source: any = {}) {
	        return new CleanResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.freed = source["freed"];
	        this.count = source["count"];
	        this.cleaned = source["cleaned"];
	        this.failed = this.convertValues(source["failed"], CleanFailure);
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
	export class DismReportDTO {
	    reportedSize: string;
	    actualSize: string;
	    reclaimablePkgs: string;
	    lastCleanup: string;
	    recommended: boolean;
	    raw: string;
	
	    static createFrom(source: any = {}) {
	        return new DismReportDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.reportedSize = source["reportedSize"];
	        this.actualSize = source["actualSize"];
	        this.reclaimablePkgs = source["reclaimablePkgs"];
	        this.lastCleanup = source["lastCleanup"];
	        this.recommended = source["recommended"];
	        this.raw = source["raw"];
	    }
	}
	export class DismInfoDTO {
	    available: boolean;
	    elevated: boolean;
	    report?: DismReportDTO;
	
	    static createFrom(source: any = {}) {
	        return new DismInfoDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.elevated = source["elevated"];
	        this.report = this.convertValues(source["report"], DismReportDTO);
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
	
	export class HistoryDTO {
	    time: string;
	    path: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new HistoryDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.path = source["path"];
	        this.size = source["size"];
	    }
	}
	export class RegEntryDTO {
	    key: string;
	    category: string;
	    risk: string;
	    desc: string;
	    values: number;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new RegEntryDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.category = source["category"];
	        this.risk = source["risk"];
	        this.desc = source["desc"];
	        this.values = source["values"];
	        this.size = source["size"];
	    }
	}
	export class UpdateInfoDTO {
	    current: string;
	    latest: string;
	    hasUpdate: boolean;
	    url: string;
	    notes: string;
	    size: number;
	    publishedAt: string;
	    fromCache: boolean;
	    snoozed: boolean;
	    skipped: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfoDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = source["current"];
	        this.latest = source["latest"];
	        this.hasUpdate = source["hasUpdate"];
	        this.url = source["url"];
	        this.notes = source["notes"];
	        this.size = source["size"];
	        this.publishedAt = source["publishedAt"];
	        this.fromCache = source["fromCache"];
	        this.snoozed = source["snoozed"];
	        this.skipped = source["skipped"];
	    }
	}

}

