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
	export class FrpcProcessInfo {
	    pids: number[];
	    killCommand: string;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new FrpcProcessInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pids = source["pids"];
	        this.killCommand = source["killCommand"];
	        this.message = source["message"];
	    }
	}

}

export namespace main {

	export class AppSettings {
	    toolPath: string;
	    configPath: string;
	    downloadUrl: string;
	    theme: string;
	    autoStart: boolean;

	    static createFrom(source: any = {}) {
	        return new AppSettings(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.toolPath = source["toolPath"];
	        this.configPath = source["configPath"];
	        this.downloadUrl = source["downloadUrl"];
	        this.theme = source["theme"];
	        this.autoStart = source["autoStart"];
	    }
	}
	export class DownloadTarget {
	    url: string;
	    filename: string;
	    version: string;

	    static createFrom(source: any = {}) {
	        return new DownloadTarget(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.filename = source["filename"];
	        this.version = source["version"];
	    }
	}
	export class SettingsFileStatus {
	    toolExists: boolean;
	    configExists: boolean;
	    toolPath: string;
	    configPath: string;
	    toolHelp: string;
	    configHelp: string;
	    downloadHelp: string;
	    manualKillHelp: string;

	    static createFrom(source: any = {}) {
	        return new SettingsFileStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.toolExists = source["toolExists"];
	        this.configExists = source["configExists"];
	        this.toolPath = source["toolPath"];
	        this.configPath = source["configPath"];
	        this.toolHelp = source["toolHelp"];
	        this.configHelp = source["configHelp"];
	        this.downloadHelp = source["downloadHelp"];
	        this.manualKillHelp = source["manualKillHelp"];
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
