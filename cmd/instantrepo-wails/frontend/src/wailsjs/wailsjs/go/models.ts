export namespace domain {
	
	export class SafetyFinding {
	    severity: string;
	    summary: string;
	    filePath?: string;
	
	    static createFrom(source: any = {}) {
	        return new SafetyFinding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.severity = source["severity"];
	        this.summary = source["summary"];
	        this.filePath = source["filePath"];
	    }
	}
	export class SafetyReport {
	    riskLevel: string;
	    findings: SafetyFinding[];
	
	    static createFrom(source: any = {}) {
	        return new SafetyReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.riskLevel = source["riskLevel"];
	        this.findings = this.convertValues(source["findings"], SafetyFinding);
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
	export class RequirementGap {
	    tool: string;
	    requiredVersion: string;
	    installedVersion?: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new RequirementGap(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tool = source["tool"];
	        this.requiredVersion = source["requiredVersion"];
	        this.installedVersion = source["installedVersion"];
	        this.status = source["status"];
	    }
	}
	export class SetupPlan {
	    projectName: string;
	    projectType: string;
	    confidence: number;
	    evidence: string[];
	    gaps: RequirementGap[];
	    env: EnvironmentConfig;
	    services: ServiceDependency[];
	    steps: ExecutionStep[];
	    safety: SafetyReport;
	    unknowns: string[];
	
	    static createFrom(source: any = {}) {
	        return new SetupPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projectName = source["projectName"];
	        this.projectType = source["projectType"];
	        this.confidence = source["confidence"];
	        this.evidence = source["evidence"];
	        this.gaps = this.convertValues(source["gaps"], RequirementGap);
	        this.env = this.convertValues(source["env"], EnvironmentConfig);
	        this.services = this.convertValues(source["services"], ServiceDependency);
	        this.steps = this.convertValues(source["steps"], ExecutionStep);
	        this.safety = this.convertValues(source["safety"], SafetyReport);
	        this.unknowns = source["unknowns"];
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
	export class DetectedTool {
	    name: string;
	    path?: string;
	    version?: string;
	    available: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DetectedTool(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.version = source["version"];
	        this.available = source["available"];
	    }
	}
	export class EnvironmentReport {
	    os: string;
	    arch: string;
	    tools: DetectedTool[];
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.os = source["os"];
	        this.arch = source["arch"];
	        this.tools = this.convertValues(source["tools"], DetectedTool);
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
	export class ExecutionStep {
	    id: string;
	    title: string;
	    command: string;
	    cwd: string;
	    type: string;
	    importance: string;
	    risk: string;
	    requiresApproval: boolean;
	    evidenceSource?: string;
	    confirmedBy?: string[];
	    confidence: number;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new ExecutionStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.command = source["command"];
	        this.cwd = source["cwd"];
	        this.type = source["type"];
	        this.importance = source["importance"];
	        this.risk = source["risk"];
	        this.requiresApproval = source["requiresApproval"];
	        this.evidenceSource = source["evidenceSource"];
	        this.confirmedBy = source["confirmedBy"];
	        this.confidence = source["confidence"];
	        this.reason = source["reason"];
	    }
	}
	export class ServiceDependency {
	    name: string;
	    scope: string;
	    provisioning: string;
	    source: string;
	    status: string;
	    details?: string;
	    instructions?: string[];
	
	    static createFrom(source: any = {}) {
	        return new ServiceDependency(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.scope = source["scope"];
	        this.provisioning = source["provisioning"];
	        this.source = source["source"];
	        this.status = source["status"];
	        this.details = source["details"];
	        this.instructions = source["instructions"];
	    }
	}
	export class EnvVarRequirement {
	    name: string;
	    source: string;
	    required: boolean;
	    secret: boolean;
	    currentStatus: string;
	    fillStrategy: string;
	    service?: string;
	    suggestedValue?: string;
	    instructions?: string[];
	
	    static createFrom(source: any = {}) {
	        return new EnvVarRequirement(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.source = source["source"];
	        this.required = source["required"];
	        this.secret = source["secret"];
	        this.currentStatus = source["currentStatus"];
	        this.fillStrategy = source["fillStrategy"];
	        this.service = source["service"];
	        this.suggestedValue = source["suggestedValue"];
	        this.instructions = source["instructions"];
	    }
	}
	export class EnvironmentConfig {
	    templatePath?: string;
	    targetPath?: string;
	    targetExists: boolean;
	    variables: EnvVarRequirement[];
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.templatePath = source["templatePath"];
	        this.targetPath = source["targetPath"];
	        this.targetExists = source["targetExists"];
	        this.variables = this.convertValues(source["variables"], EnvVarRequirement);
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
	export class ToolRequirement {
	    tool: string;
	    versionConstraint: string;
	    source: string;
	    required: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ToolRequirement(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tool = source["tool"];
	        this.versionConstraint = source["versionConstraint"];
	        this.source = source["source"];
	        this.required = source["required"];
	    }
	}
	export class RepositoryAnalysis {
	    projectName: string;
	    projectType: string;
	    repoPath: string;
	    confidence: number;
	    evidence: string[];
	    requirements: ToolRequirement[];
	    env: EnvironmentConfig;
	    services: ServiceDependency[];
	    steps: ExecutionStep[];
	    unknowns: string[];
	
	    static createFrom(source: any = {}) {
	        return new RepositoryAnalysis(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projectName = source["projectName"];
	        this.projectType = source["projectType"];
	        this.repoPath = source["repoPath"];
	        this.confidence = source["confidence"];
	        this.evidence = source["evidence"];
	        this.requirements = this.convertValues(source["requirements"], ToolRequirement);
	        this.env = this.convertValues(source["env"], EnvironmentConfig);
	        this.services = this.convertValues(source["services"], ServiceDependency);
	        this.steps = this.convertValues(source["steps"], ExecutionStep);
	        this.unknowns = source["unknowns"];
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
	export class RepoSource {
	    type: string;
	    repoUrl?: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new RepoSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.repoUrl = source["repoUrl"];
	        this.path = source["path"];
	    }
	}
	export class AnalyzeResponse {
	    source: RepoSource;
	    analysis: RepositoryAnalysis;
	    environment: EnvironmentReport;
	    plan: SetupPlan;
	
	    static createFrom(source: any = {}) {
	        return new AnalyzeResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = this.convertValues(source["source"], RepoSource);
	        this.analysis = this.convertValues(source["analysis"], RepositoryAnalysis);
	        this.environment = this.convertValues(source["environment"], EnvironmentReport);
	        this.plan = this.convertValues(source["plan"], SetupPlan);
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
	
	
	
	
	export class ExecutionResult {
	    stepId: string;
	    command: string;
	    cwd: string;
	    processId: number;
	    exitCode: number;
	    stdout: string;
	    stderr: string;
	    duration: string;
	    succeeded: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ExecutionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stepId = source["stepId"];
	        this.command = source["command"];
	        this.cwd = source["cwd"];
	        this.processId = source["processId"];
	        this.exitCode = source["exitCode"];
	        this.stdout = source["stdout"];
	        this.stderr = source["stderr"];
	        this.duration = source["duration"];
	        this.succeeded = source["succeeded"];
	    }
	}
	export class ExecuteResponse {
	    source: RepoSource;
	    analysis: RepositoryAnalysis;
	    environment: EnvironmentReport;
	    plan: SetupPlan;
	    result: ExecutionResult;
	
	    static createFrom(source: any = {}) {
	        return new ExecuteResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = this.convertValues(source["source"], RepoSource);
	        this.analysis = this.convertValues(source["analysis"], RepositoryAnalysis);
	        this.environment = this.convertValues(source["environment"], EnvironmentReport);
	        this.plan = this.convertValues(source["plan"], SetupPlan);
	        this.result = this.convertValues(source["result"], ExecutionResult);
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

