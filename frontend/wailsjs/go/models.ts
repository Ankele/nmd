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

}

