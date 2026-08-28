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
		    if (a.slice) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    }
		    return new classs(a);
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

}

