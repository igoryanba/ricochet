/**
 * Session State Machine
 * 
 * Manages the lifecycle state of an agent session from creation to completion.
 */

// ============ States ============
export const SessionState = {
    idle: "idle",
    creating: "creating",
    streaming: "streaming",
    waiting_approval: "waiting_approval",
    waiting_input: "waiting_input",
    completed: "completed",
    paused: "paused",
    error: "error",
    stopped: "stopped",
} as const

export type SessionState = (typeof SessionState)[keyof typeof SessionState]

// ============ Events ============

type StartSessionEvent = { type: "start_session" }
type SessionCreatedEvent = { type: "session_created"; sessionId: string }
type ApiReqStartedEvent = { type: "api_req_started" }
type SayTextEvent = { type: "say_text"; partial?: boolean; payload?: { text: string } }
type AskToolEvent = { type: "ask_tool"; partial?: boolean; payload?: { name: string; args: any; toolId: string } }
type AskCommandEvent = { type: "ask_command"; partial: boolean }
type AskBrowserActionLaunchEvent = { type: "ask_browser_action_launch"; partial: boolean }
type AskUseMcpServerEvent = { type: "ask_use_mcp_server"; partial: boolean }
type AskFollowupEvent = { type: "ask_followup"; partial: boolean }
type AskCompletionResultEvent = { type: "ask_completion_result" }
type AskApiReqFailedEvent = { type: "ask_api_req_failed" }
type AskMistakeLimitReachedEvent = { type: "ask_mistake_limit_reached" }
type AskInvalidModelEvent = { type: "ask_invalid_model" }
type AskPaymentRequiredPromptEvent = { type: "ask_payment_required_prompt" }
type AskResumeTaskEvent = { type: "ask_resume_task" }
type ProcessErrorEvent = { type: "process_error"; error: string }
type ApproveActionEvent = { type: "approve_action" }
type RejectActionEvent = { type: "reject_action" }
type SendMessageEvent = { type: "send_message"; content: string }
type CancelSessionEvent = { type: "cancel_session" }
type RetryEvent = { type: "retry" }

export type SessionEvent =
    | StartSessionEvent
    | SessionCreatedEvent
    | ApiReqStartedEvent
    | SayTextEvent
    | AskToolEvent
    | AskCommandEvent
    | AskBrowserActionLaunchEvent
    | AskUseMcpServerEvent
    | AskFollowupEvent
    | AskCompletionResultEvent
    | AskApiReqFailedEvent
    | AskMistakeLimitReachedEvent
    | AskInvalidModelEvent
    | AskPaymentRequiredPromptEvent
    | AskResumeTaskEvent
    | ProcessErrorEvent
    | ApproveActionEvent
    | RejectActionEvent
    | SendMessageEvent
    | CancelSessionEvent
    | CancelSessionEvent
    | RetryEvent

export interface SessionUiState {
    showSpinner: boolean
    showCancelButton: boolean
    isActive: boolean
}

export interface SessionStateMachine {
    getState: () => SessionState
    send: (event: SessionEvent) => void
    getUiState: () => SessionUiState
    getContext: () => SessionContext
    reset: () => void
}

// ============ Context ============

// ============ Context ============
export interface AgentLogEntry {
    id: string;
    timestamp: number;
    type: 'info' | 'tool_call' | 'tool_result' | 'error' | 'user' | 'step';
    content: string;
    metadata?: any;
}

export interface SessionContext {
    sessionId?: string
    errorMessage?: string
    sawApiReqStarted: boolean
    sawSessionCreated: boolean
    logs: AgentLogEntry[]
}

// ...

function createInitialContext(): SessionContext {
    return {
        sessionId: undefined,
        errorMessage: undefined,
        sawApiReqStarted: false,
        sawSessionCreated: false,
        logs: [],
    }
}

export function createSessionStateMachine(): SessionStateMachine {
    let state: SessionState = SessionState.idle
    let context: SessionContext = createInitialContext()

    const send = (event: SessionEvent): void => {
        const { nextState, contextUpdate } = transition(state, event, context)
        state = nextState
        if (contextUpdate) {
            // Merge simple fields
            const { logs, ...otherUpdates } = contextUpdate;
            context = { ...context, ...otherUpdates }

            // Append Logs if any (array merge)
            if (logs) {
                context.logs = [...context.logs, ...logs];
            }
        }
    }

    const getState = (): SessionState => state

    const getUiState = (): SessionUiState => {
        return {
            showSpinner: state === SessionState.creating || state === SessionState.streaming,
            showCancelButton:
                state === SessionState.creating ||
                state === SessionState.streaming ||
                state === SessionState.waiting_approval ||
                state === SessionState.waiting_input,
            isActive:
                state === SessionState.creating ||
                state === SessionState.streaming ||
                state === SessionState.waiting_approval ||
                state === SessionState.waiting_input,
        }
    }

    const getContext = (): SessionContext => ({ ...context })

    const reset = (): void => {
        state = SessionState.idle
        context = createInitialContext()
    }

    return {
        getState,
        send,
        getUiState,
        getContext,
        reset,
    }
}

// ============ Transition Logic ============

interface TransitionResult {
    nextState: SessionState
    contextUpdate?: Partial<SessionContext>
}

function transition(currentState: SessionState, event: SessionEvent, context: SessionContext): TransitionResult {
    switch (currentState) {
        case SessionState.idle:
            return transitionFromIdle(event)

        case SessionState.creating:
            return transitionFromCreating(event, context)

        case SessionState.streaming:
            return transitionFromStreaming(event)

        case SessionState.waiting_approval:
            return transitionFromWaitingApproval(event)

        case SessionState.waiting_input:
            return transitionFromWaitingInput(event)

        case SessionState.completed:
            return transitionFromCompleted(event)

        case SessionState.paused:
            return transitionFromPaused(event)

        case SessionState.error:
            return transitionFromError(event)

        case SessionState.stopped:
            return transitionFromStopped(event)

        default:
            return { nextState: currentState }
    }
}

function transitionFromIdle(event: SessionEvent): TransitionResult {
    switch (event.type) {
        case "start_session":
            return { nextState: SessionState.creating }

        // Allow direct transition to streaming if we receive events that indicate
        // the session is already running
        case "session_created":
            return {
                nextState: SessionState.streaming,
                contextUpdate: {
                    sawSessionCreated: true,
                    sessionId: (event as SessionCreatedEvent).sessionId,
                },
            }

        case "api_req_started":
            return {
                nextState: SessionState.streaming,
                contextUpdate: { sawApiReqStarted: true },
            }

        default:
            return { nextState: SessionState.idle }
    }
}

function transitionFromCreating(event: SessionEvent, context: SessionContext): TransitionResult {
    switch (event.type) {
        case "session_created": {
            const newContext: Partial<SessionContext> = {
                sawSessionCreated: true,
                sessionId: event.sessionId,
            }
            // Transition to streaming only if we've seen both events
            if (context.sawApiReqStarted) {
                return { nextState: SessionState.streaming, contextUpdate: newContext }
            }
            return { nextState: SessionState.creating, contextUpdate: newContext }
        }

        case "api_req_started": {
            const newContext: Partial<SessionContext> = { sawApiReqStarted: true }
            // Transition to streaming only if we've seen both events
            if (context.sawSessionCreated) {
                return { nextState: SessionState.streaming, contextUpdate: newContext }
            }
            return { nextState: SessionState.creating, contextUpdate: newContext }
        }

        case "process_error":
            return {
                nextState: SessionState.error,
                contextUpdate: { errorMessage: event.error },
            }

        case "cancel_session":
            return { nextState: SessionState.stopped }

        default:
            return { nextState: SessionState.creating }
    }
}

function transitionFromStreaming(event: SessionEvent): TransitionResult {
    const timestamp = Date.now();
    switch (event.type) {
        // Stay streaming on any say message
        case "say_text":
            return {
                nextState: SessionState.streaming,
                contextUpdate: {
                    logs: [{
                        id: `log-${timestamp}`,
                        timestamp,
                        type: 'info',
                        content: (event as SayTextEvent).payload?.text || "", // Verify event structure
                    }]
                }
            }

        // Stay streaming on api_req_started (new request)
        case "api_req_started":
            return { nextState: SessionState.streaming }

        // Approval-required asks (only on complete)
        case "ask_tool":
            return {
                nextState: event.partial ? SessionState.streaming : SessionState.waiting_approval,
                contextUpdate: event.partial ? undefined : {
                    logs: [{
                        id: `log-${timestamp}`,
                        timestamp,
                        type: 'tool_call',
                        content: (event as any).payload?.name || "Tool Call", // Using any cast for quick fix, need to verify types
                        metadata: (event as any).payload
                    }]
                }
            }

        case "ask_command":
        case "ask_browser_action_launch":
        case "ask_use_mcp_server":
            if (event.partial) {
                return { nextState: SessionState.streaming }
            }
            return { nextState: SessionState.waiting_approval }

        // Input-required asks (only on complete)
        case "ask_followup":
            return {
                nextState: SessionState.waiting_input,
                contextUpdate: event.partial ? undefined : {
                    logs: [{
                        id: `log-${timestamp}`,
                        timestamp,
                        type: 'info',
                        content: "Waiting for user input..."
                    }]
                }
            }

        // Completion
        case "ask_completion_result":
            return {
                nextState: SessionState.completed,
                contextUpdate: {
                    logs: [{
                        id: `log-${timestamp}`,
                        timestamp,
                        type: 'info',
                        content: "Task Completed."
                    }]
                }
            }

        // Errors
        case "ask_api_req_failed":
        case "ask_mistake_limit_reached":
        case "ask_invalid_model":
        case "ask_payment_required_prompt":
            return {
                nextState: SessionState.error,
                contextUpdate: {
                    logs: [{
                        id: `log-${timestamp}`,
                        timestamp,
                        type: 'error',
                        content: "An error occurred during execution."
                    }]
                }
            }

        // Paused
        case "ask_resume_task":
            return { nextState: SessionState.paused }

        // Cancel
        case "cancel_session":
            return {
                nextState: SessionState.stopped,
                contextUpdate: {
                    logs: [{
                        id: `log-${timestamp}`,
                        timestamp,
                        type: 'info',
                        content: "Session stopped by user."
                    }]
                }
            }

        default:
            return { nextState: SessionState.streaming }
    }
}

function transitionFromWaitingApproval(event: SessionEvent): TransitionResult {
    switch (event.type) {
        case "approve_action":
        case "reject_action":
        case "api_req_started": // Auto-approved
            return { nextState: SessionState.streaming }

        case "cancel_session":
            return { nextState: SessionState.stopped }

        default:
            return { nextState: SessionState.waiting_approval }
    }
}

function transitionFromWaitingInput(event: SessionEvent): TransitionResult {
    switch (event.type) {
        case "api_req_started":
            return { nextState: SessionState.streaming }

        case "send_message":
            return { nextState: SessionState.streaming }

        case "cancel_session":
            return { nextState: SessionState.stopped }

        default:
            return { nextState: SessionState.waiting_input }
    }
}

function transitionFromCompleted(event: SessionEvent): TransitionResult {
    switch (event.type) {
        case "api_req_started":
            return { nextState: SessionState.streaming }

        case "send_message": // Continue with follow-up
            return { nextState: SessionState.streaming }

        case "start_session": // New task
            return { nextState: SessionState.creating }

        default:
            return { nextState: SessionState.completed }
    }
}

function transitionFromPaused(event: SessionEvent): TransitionResult {
    switch (event.type) {
        case "api_req_started":
            return { nextState: SessionState.streaming }

        case "cancel_session":
            return { nextState: SessionState.stopped }

        default:
            return { nextState: SessionState.paused }
    }
}

function transitionFromError(event: SessionEvent): TransitionResult {
    switch (event.type) {
        case "api_req_started":
            return { nextState: SessionState.streaming }

        case "retry":
            return { nextState: SessionState.streaming }

        case "cancel_session":
            return { nextState: SessionState.stopped }

        default:
            return { nextState: SessionState.error }
    }
}

function transitionFromStopped(event: SessionEvent): TransitionResult {
    switch (event.type) {
        case "api_req_started":
            return { nextState: SessionState.streaming }

        case "start_session": // New task
            return { nextState: SessionState.creating }

        default:
            return { nextState: SessionState.stopped }
    }
}
