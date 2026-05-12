export namespace download {

	export class Mirror {
	    name: string;
	    type: string;
	    template: string;

	    static createFrom(source: any = {}) {
	        return new Mirror(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.template = source["template"];
	    }
	}

}

export namespace frp {

	export class FrpStatus {
	    running: boolean;
	    pid: number;
	    uptime: string;
	    version: string;
	    logPath: string;
	    configPath: string;
	    binaryPath: string;

	    static createFrom(source: any = {}) {
	        return new FrpStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.pid = source["pid"];
	        this.uptime = source["uptime"];
	        this.version = source["version"];
	        this.logPath = source["logPath"];
	        this.configPath = source["configPath"];
	        this.binaryPath = source["binaryPath"];
	    }
	}

}

export namespace main {

	export class AppSettings {
	    rootDir: string;
	    toolsDir: string;
	    theme: string;
	    autoStart: boolean;

	    static createFrom(source: any = {}) {
	        return new AppSettings(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rootDir = source["rootDir"];
	        this.toolsDir = source["toolsDir"];
	        this.theme = source["theme"];
	        this.autoStart = source["autoStart"];
	    }
	}

}

export namespace system {

	export class SystemInfo {
	    os: string;
	    arch: string;

	    static createFrom(source: any = {}) {
	        return new SystemInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.os = source["os"];
	        this.arch = source["arch"];
	    }
	}

}
