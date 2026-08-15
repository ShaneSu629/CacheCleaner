export namespace main {
	
	export class CleanResult {
	    freed: number;
	    count: number;
	    failed: string[];
	
	    static createFrom(source: any = {}) {
	        return new CleanResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.freed = source["freed"];
	        this.count = source["count"];
	        this.failed = source["failed"];
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

