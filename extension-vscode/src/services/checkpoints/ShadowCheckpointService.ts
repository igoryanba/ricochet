import fs from "fs/promises"
import * as path from "path"
import crypto from "crypto"
import EventEmitter from "events"
import simpleGit, { SimpleGit, SimpleGitOptions } from "simple-git"
import * as vscode from "vscode"
import { CheckpointDiff, CheckpointResult, CheckpointEventMap } from "./types"
import { getExcludePatterns } from "./excludes"

// Simple p-wait-for replacement
const waitFor = async (condition: () => Promise<boolean> | boolean, options: { interval: number, timeout: number }) => {
    const start = Date.now();
    while (Date.now() - start < options.timeout) {
        if (await condition()) return;
        await new Promise(resolve => setTimeout(resolve, options.interval));
    }
    throw new Error("Timeout waiting for condition");
};

async function fileExistsAtPath(path: string) {
    try {
        await fs.access(path)
        return true
    } catch {
        return false
    }
}

function createSanitizedGit(baseDir: string): SimpleGit {
    const sanitizedEnv: Record<string, string> = {}
    const removedVars: string[] = []

    for (const [key, value] of Object.entries(process.env)) {
        if (
            key === "GIT_DIR" ||
            key === "GIT_WORK_TREE" ||
            key === "GIT_INDEX_FILE" ||
            key === "GIT_OBJECT_DIRECTORY" ||
            key === "GIT_ALTERNATE_OBJECT_DIRECTORIES" ||
            key === "GIT_CEILING_DIRECTORIES"
        ) {
            removedVars.push(`${key}=${value}`)
            continue
        }
        if (value !== undefined) {
            sanitizedEnv[key] = value
        }
    }

    if (removedVars.length > 0) {
        console.log(`[createSanitizedGit] Removed git env vars: ${removedVars.join(", ")}`)
    }

    const options: Partial<SimpleGitOptions> = {
        baseDir,
        config: [],
    }

    const git = simpleGit(options)
    git.env(sanitizedEnv)
    return git
}

export class ShadowCheckpointService extends EventEmitter {
    public readonly taskId: string
    public readonly checkpointsDir: string
    public readonly workspaceDir: string

    protected _checkpoints: string[] = []
    protected _baseHash?: string

    protected readonly dotGitDir: string
    protected git?: SimpleGit
    protected readonly log: (message: string) => void
    protected shadowGitConfigWorktree?: string

    public get baseHash() {
        return this._baseHash
    }

    protected set baseHash(value: string | undefined) {
        this._baseHash = value
    }

    public get isInitialized() {
        return !!this.git
    }

    public getCheckpoints(): string[] {
        return this._checkpoints.slice()
    }

    constructor(taskId: string, checkpointsDir: string, workspaceDir: string, log: (message: string) => void) {
        super()
        this.taskId = taskId
        this.checkpointsDir = checkpointsDir
        this.workspaceDir = workspaceDir
        this.dotGitDir = path.join(this.checkpointsDir, ".git")
        this.log = log
    }

    public async initShadowGit(onInit?: () => Promise<void>) {
        if (this.git) {
            throw new Error("Shadow git repo already initialized")
        }

        // Simplified nested git check: just warn if .git exists in workspace but isn't root
        // For now, we assume user is responsible.
        // Full check could be added later.

        await fs.mkdir(this.checkpointsDir, { recursive: true })
        const git = createSanitizedGit(this.checkpointsDir)

        let created = false
        const startTime = Date.now()

        if (await fileExistsAtPath(this.dotGitDir)) {
            this.log(`[ShadowCheckpointService] using existing shadow git repo at ${this.dotGitDir}`)
            const worktree = (await git.getConfig("core.worktree")).value || undefined
            this.shadowGitConfigWorktree = worktree

            if (worktree !== this.workspaceDir) {
                throw new Error(
                    `Checkpoints can only be used in the original workspace: ${worktree} !== ${this.workspaceDir}`,
                )
            }

            await this.writeExcludeFile()
            this.baseHash = await git.revparse(["HEAD"])
        } else {
            this.log(`[ShadowCheckpointService] creating shadow git repo at ${this.checkpointsDir}`)
            await git.init()
            await git.addConfig("core.worktree", this.workspaceDir)
            await git.addConfig("commit.gpgSign", "false")
            await git.addConfig("user.name", "Ricochet")
            await git.addConfig("user.email", "noreply@ricochet.ai")
            await this.writeExcludeFile()
            await this.stageAll(git)
            const { commit } = await git.commit("initial commit", { "--allow-empty": null })
            this.baseHash = commit
            created = true
        }

        const duration = Date.now() - startTime
        this.log(`[ShadowCheckpointService] initialized shadow repo with base commit ${this.baseHash} in ${duration}ms`)
        this.git = git
        await onInit?.()

        this.emit("initialize", {
            type: "initialize",
            workspaceDir: this.workspaceDir,
            baseHash: this.baseHash,
            created,
            duration,
        })
    }

    protected async writeExcludeFile() {
        await fs.mkdir(path.join(this.dotGitDir, "info"), { recursive: true })
        const patterns = await getExcludePatterns(this.workspaceDir)
        await fs.writeFile(path.join(this.dotGitDir, "info", "exclude"), patterns.join("\n"))
    }

    private async stageAll(git: SimpleGit) {
        try {
            await git.add([".", "--ignore-errors"])
        } catch (error) {
            this.log(`[ShadowCheckpointService] failed to add files: ${error}`)
        }
    }

    public async saveCheckpoint(message: string): Promise<CheckpointResult | undefined> {
        if (!this.git) throw new Error("Shadow git repo not initialized")

        const startTime = Date.now()
        await this.stageAll(this.git)
        const result = await this.git.commit(message)
        const fromHash = this._checkpoints[this._checkpoints.length - 1] ?? this.baseHash!
        const toHash = result.commit || fromHash
        if (result.commit) {
            this._checkpoints.push(toHash)
        }

        const duration = Date.now() - startTime

        if (result.commit) {
            this.emit("checkpoint", {
                type: "checkpoint",
                fromHash,
                toHash,
                duration,
            })
            return result
        }
        return undefined
    }

    public async restoreCheckpoint(commitHash: string) {
        if (!this.git) throw new Error("Shadow git repo not initialized")

        const start = Date.now()
        await this.git.clean("f", ["-d", "-f"])
        await this.git.reset(["--hard", commitHash])

        const checkpointIndex = this._checkpoints.indexOf(commitHash)
        if (checkpointIndex !== -1) {
            this._checkpoints = this._checkpoints.slice(0, checkpointIndex + 1)
        }

        const duration = Date.now() - start
        this.emit("restore", { type: "restore", commitHash, duration })
    }

    public async getDiff({ from, to }: { from?: string; to?: string }): Promise<CheckpointDiff[]> {
        if (!this.git) throw new Error("Shadow git repo not initialized")

        if (!from) {
            from = (await this.git.raw(["rev-list", "--max-parents=0", "HEAD"])).trim()
        }

        await this.stageAll(this.git)
        const { files } = to ? await this.git.diffSummary([`${from}..${to}`]) : await this.git.diffSummary([from])

        const cwdPath = this.shadowGitConfigWorktree || this.workspaceDir || ""
        const result: CheckpointDiff[] = []

        for (const file of files) {
            const relPath = file.file
            const absPath = path.join(cwdPath, relPath)
            const before = await this.git.show([`${from}:${relPath}`]).catch(() => "")
            const after = to
                ? await this.git.show([`${to}:${relPath}`]).catch(() => "")
                : await fs.readFile(absPath, "utf8").catch(() => "")

            result.push({ paths: { relative: relPath, absolute: absPath }, content: { before, after } })
        }
        return result
    }
}
