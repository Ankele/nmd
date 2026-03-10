export namespace main {
	
	export class ExportPDFRequest {
	    path: string;
	    fileName: string;
	    content: string;
	    documentPath: string;
	
	    static createFrom(source: any = {}) {
	        return new ExportPDFRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.fileName = source["fileName"];
	        this.content = source["content"];
	        this.documentPath = source["documentPath"];
	    }
	}
	export class ExportPDFResult {
	    path: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new ExportPDFResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	    }
	}
	export class OpenFileResult {
	    path: string;
	    name: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new OpenFileResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.content = source["content"];
	    }
	}
	export class RecentFile {
	    path: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new RecentFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	    }
	}
	export class SaveFileRequest {
	    path: string;
	    fileName: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new SaveFileRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.fileName = source["fileName"];
	        this.content = source["content"];
	    }
	}
	export class SaveFileResult {
	    path: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new SaveFileResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	    }
	}
	export class SaveImageAssetRequest {
	    documentPath: string;
	    fileName: string;
	    dataURL: string;
	
	    static createFrom(source: any = {}) {
	        return new SaveImageAssetRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.documentPath = source["documentPath"];
	        this.fileName = source["fileName"];
	        this.dataURL = source["dataURL"];
	    }
	}
	export class SaveImageAssetResult {
	    absolutePath: string;
	    relativePath: string;
	
	    static createFrom(source: any = {}) {
	        return new SaveImageAssetResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.absolutePath = source["absolutePath"];
	        this.relativePath = source["relativePath"];
	    }
	}
	export class WorkspaceEntry {
	    name: string;
	    path: string;
	    isDir: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.isDir = source["isDir"];
	    }
	}
	export class WorkspaceSearchHit {
	    path: string;
	    line: number;
	    column: number;
	    preview: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceSearchHit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.line = source["line"];
	        this.column = source["column"];
	        this.preview = source["preview"];
	    }
	}
	export class WorkspaceReplaceResult {
	    filesChanged: number;
	    occurrences: number;
	    paths: string[];
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceReplaceResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filesChanged = source["filesChanged"];
	        this.occurrences = source["occurrences"];
	        this.paths = source["paths"];
	    }
	}
	export class WorkspaceReplacePreviewItem {
	    path: string;
	    occurrences: number;
	    sample: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceReplacePreviewItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.occurrences = source["occurrences"];
	        this.sample = source["sample"];
	    }
	}
	export class WorkspaceReplacePreviewResult {
	    files: number;
	    occurrences: number;
	    items: WorkspaceReplacePreviewItem[];
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceReplacePreviewResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = source["files"];
	        this.occurrences = source["occurrences"];
	        this.items = this.convertValues(source["items"], WorkspaceReplacePreviewItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if(!a) {
		        return a;
		    }
		    if (a.slice && !asMap) {
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
