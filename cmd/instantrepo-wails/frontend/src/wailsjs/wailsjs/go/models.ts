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
	    targetDir?: string;
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
	        this.targetDir = source["targetDir"];
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
	export class CloneDiskStatus {
	    status: string;
	    freeBytes?: number;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new CloneDiskStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.freeBytes = source["freeBytes"];
	        this.reason = source["reason"];
	    }
	}
	export class ClonePreflightMessage {
	    severity: string;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new ClonePreflightMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.severity = source["severity"];
	        this.text = source["text"];
	    }
	}
	export class InstalledRepo {
	    id: number;
	    rawUrl?: string;
	    normalizedUrl?: string;
	    localPath: string;
	    status: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    // Go type: time
	    lastAnalyzedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new InstalledRepo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.rawUrl = source["rawUrl"];
	        this.normalizedUrl = source["normalizedUrl"];
	        this.localPath = source["localPath"];
	        this.status = source["status"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.lastAnalyzedAt = this.convertValues(source["lastAnalyzedAt"], null);
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
	export class ClonePreflightResponse {
	    repoUrl: string;
	    normalizedUrl: string;
	    destinationRoot: string;
	    destinationWritable: boolean;
	    targetPath: string;
	    targetExists: boolean;
	    targetEmpty: boolean;
	    duplicateRepos: InstalledRepo[];
	    pathConflict: boolean;
	    pathConflictRepos: InstalledRepo[];
	    disk: CloneDiskStatus;
	    recommendedAction: string;
	    messages: ClonePreflightMessage[];
	
	    static createFrom(source: any = {}) {
	        return new ClonePreflightResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repoUrl = source["repoUrl"];
	        this.normalizedUrl = source["normalizedUrl"];
	        this.destinationRoot = source["destinationRoot"];
	        this.destinationWritable = source["destinationWritable"];
	        this.targetPath = source["targetPath"];
	        this.targetExists = source["targetExists"];
	        this.targetEmpty = source["targetEmpty"];
	        this.duplicateRepos = this.convertValues(source["duplicateRepos"], InstalledRepo);
	        this.pathConflict = source["pathConflict"];
	        this.pathConflictRepos = this.convertValues(source["pathConflictRepos"], InstalledRepo);
	        this.disk = this.convertValues(source["disk"], CloneDiskStatus);
	        this.recommendedAction = source["recommendedAction"];
	        this.messages = this.convertValues(source["messages"], ClonePreflightMessage);
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
	
	
	
	export class SetupSessionSummary {
	    id: number;
	    installedRepoId: number;
	    repoPath: string;
	    status: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new SetupSessionSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.installedRepoId = source["installedRepoId"];
	        this.repoPath = source["repoPath"];
	        this.status = source["status"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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
	export class InstalledRepoSummary {
	    id: number;
	    projectName: string;
	    localPath: string;
	    status: string;
	    // Go type: time
	    lastAnalyzedAt: any;
	    // Go type: time
	    lastSetupAt: any;
	    // Go type: time
	    lastActivityAt: any;
	
	    static createFrom(source: any = {}) {
	        return new InstalledRepoSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.projectName = source["projectName"];
	        this.localPath = source["localPath"];
	        this.status = source["status"];
	        this.lastAnalyzedAt = this.convertValues(source["lastAnalyzedAt"], null);
	        this.lastSetupAt = this.convertValues(source["lastSetupAt"], null);
	        this.lastActivityAt = this.convertValues(source["lastActivityAt"], null);
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
	export class InstalledRepoDetailsResponse {
	    repo: InstalledRepoSummary;
	    setupSessions: SetupSessionSummary[];
	
	    static createFrom(source: any = {}) {
	        return new InstalledRepoDetailsResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repo = this.convertValues(source["repo"], InstalledRepoSummary);
	        this.setupSessions = this.convertValues(source["setupSessions"], SetupSessionSummary);
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
	export class InstalledRepoManagerResponse {
	    repos: InstalledRepoSummary[];
	
	    static createFrom(source: any = {}) {
	        return new InstalledRepoManagerResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repos = this.convertValues(source["repos"], InstalledRepoSummary);
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
	
	export class RepoDiagnosticAIReviewEntryMetadata {
	    id: number;
	    commandHash?: string;
	    decision?: string;
	    confidence?: number;
	    // Go type: time
	    createdAt?: any;
	
	    static createFrom(source: any = {}) {
	        return new RepoDiagnosticAIReviewEntryMetadata(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.commandHash = source["commandHash"];
	        this.decision = source["decision"];
	        this.confidence = source["confidence"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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
	export class RepoDiagnosticAIReviewMetadata {
	    available: boolean;
	    entries: RepoDiagnosticAIReviewEntryMetadata[];
	
	    static createFrom(source: any = {}) {
	        return new RepoDiagnosticAIReviewMetadata(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.entries = this.convertValues(source["entries"], RepoDiagnosticAIReviewEntryMetadata);
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
	export class RepoDiagnosticEnvVar {
	    name: string;
	    source: string;
	    required: boolean;
	    secret: boolean;
	    currentStatus: string;
	    service?: string;
	    targetDir?: string;
	
	    static createFrom(source: any = {}) {
	        return new RepoDiagnosticEnvVar(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.source = source["source"];
	        this.required = source["required"];
	        this.secret = source["secret"];
	        this.currentStatus = source["currentStatus"];
	        this.service = source["service"];
	        this.targetDir = source["targetDir"];
	    }
	}
	export class RepoDiagnosticAnalysisSummary {
	    projectName: string;
	    projectType: string;
	    confidence: number;
	    evidence: string[];
	    requirements: ToolRequirement[];
	    envVariables: RepoDiagnosticEnvVar[];
	    services: ServiceDependency[];
	    unknowns: string[];
	
	    static createFrom(source: any = {}) {
	        return new RepoDiagnosticAnalysisSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projectName = source["projectName"];
	        this.projectType = source["projectType"];
	        this.confidence = source["confidence"];
	        this.evidence = source["evidence"];
	        this.requirements = this.convertValues(source["requirements"], ToolRequirement);
	        this.envVariables = this.convertValues(source["envVariables"], RepoDiagnosticEnvVar);
	        this.services = this.convertValues(source["services"], ServiceDependency);
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
	export class RepoDiagnosticAppInfo {
	    name: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new RepoDiagnosticAppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	    }
	}
	
	export class RepoDiagnosticEnvironment {
	    os: string;
	    arch: string;
	    tools: DetectedTool[];
	
	    static createFrom(source: any = {}) {
	        return new RepoDiagnosticEnvironment(source);
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
	export class RepoDiagnosticStep {
	    id: number;
	    setupSessionId: number;
	    stepId: string;
	    title: string;
	    commandHash: string;
	    commandPreview: string;
	    cwd: string;
	    status: string;
	    exitCode: number;
	    duration: string;
	    log?: string;
	    // Go type: time
	    startedAt: any;
	    // Go type: time
	    finishedAt: any;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new RepoDiagnosticStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.setupSessionId = source["setupSessionId"];
	        this.stepId = source["stepId"];
	        this.title = source["title"];
	        this.commandHash = source["commandHash"];
	        this.commandPreview = source["commandPreview"];
	        this.cwd = source["cwd"];
	        this.status = source["status"];
	        this.exitCode = source["exitCode"];
	        this.duration = source["duration"];
	        this.log = source["log"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.finishedAt = this.convertValues(source["finishedAt"], null);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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
	export class RepoDiagnosticSetupSession {
	    id: number;
	    installedRepoId: number;
	    repoPath: string;
	    status: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    steps: RepoDiagnosticStep[];
	
	    static createFrom(source: any = {}) {
	        return new RepoDiagnosticSetupSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.installedRepoId = source["installedRepoId"];
	        this.repoPath = source["repoPath"];
	        this.status = source["status"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.steps = this.convertValues(source["steps"], RepoDiagnosticStep);
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
	export class RepoDiagnosticPlanStep {
	    id: string;
	    title: string;
	    commandPreview: string;
	    type: string;
	    importance: string;
	    risk: string;
	    requiresApproval: boolean;
	    evidenceSource?: string;
	    confidence: number;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new RepoDiagnosticPlanStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.commandPreview = source["commandPreview"];
	        this.type = source["type"];
	        this.importance = source["importance"];
	        this.risk = source["risk"];
	        this.requiresApproval = source["requiresApproval"];
	        this.evidenceSource = source["evidenceSource"];
	        this.confidence = source["confidence"];
	        this.reason = source["reason"];
	    }
	}
	export class RepoDiagnosticSetupPlanSummary {
	    projectName: string;
	    projectType: string;
	    confidence: number;
	    evidence: string[];
	    gaps: RequirementGap[];
	    envVariables: RepoDiagnosticEnvVar[];
	    services: ServiceDependency[];
	    steps: RepoDiagnosticPlanStep[];
	    safety: SafetyReport;
	    unknowns: string[];
	
	    static createFrom(source: any = {}) {
	        return new RepoDiagnosticSetupPlanSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projectName = source["projectName"];
	        this.projectType = source["projectType"];
	        this.confidence = source["confidence"];
	        this.evidence = source["evidence"];
	        this.gaps = this.convertValues(source["gaps"], RequirementGap);
	        this.envVariables = this.convertValues(source["envVariables"], RepoDiagnosticEnvVar);
	        this.services = this.convertValues(source["services"], ServiceDependency);
	        this.steps = this.convertValues(source["steps"], RepoDiagnosticPlanStep);
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
	export class RepoDiagnosticRepoIdentity {
	    id: number;
	    rawUrl?: string;
	    normalizedUrl?: string;
	    localPath: string;
	    status: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    // Go type: time
	    lastAnalyzedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new RepoDiagnosticRepoIdentity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.rawUrl = source["rawUrl"];
	        this.normalizedUrl = source["normalizedUrl"];
	        this.localPath = source["localPath"];
	        this.status = source["status"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.lastAnalyzedAt = this.convertValues(source["lastAnalyzedAt"], null);
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
	export class RepoDiagnosticExport {
	    schemaVersion: string;
	    // Go type: time
	    generatedAt: any;
	    repo: RepoDiagnosticRepoIdentity;
	    app: RepoDiagnosticAppInfo;
	    environment: RepoDiagnosticEnvironment;
	    analysis: RepoDiagnosticAnalysisSummary;
	    setupPlan: RepoDiagnosticSetupPlanSummary;
	    setupSessions: RepoDiagnosticSetupSession[];
	    aiReview: RepoDiagnosticAIReviewMetadata;
	
	    static createFrom(source: any = {}) {
	        return new RepoDiagnosticExport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.generatedAt = this.convertValues(source["generatedAt"], null);
	        this.repo = this.convertValues(source["repo"], RepoDiagnosticRepoIdentity);
	        this.app = this.convertValues(source["app"], RepoDiagnosticAppInfo);
	        this.environment = this.convertValues(source["environment"], RepoDiagnosticEnvironment);
	        this.analysis = this.convertValues(source["analysis"], RepoDiagnosticAnalysisSummary);
	        this.setupPlan = this.convertValues(source["setupPlan"], RepoDiagnosticSetupPlanSummary);
	        this.setupSessions = this.convertValues(source["setupSessions"], RepoDiagnosticSetupSession);
	        this.aiReview = this.convertValues(source["aiReview"], RepoDiagnosticAIReviewMetadata);
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

