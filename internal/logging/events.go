package logging

// Event types for structured logging.
// These constants define the event names used in JSONL traces.
const (
	// Session events
	EventSessionStart  = "session.start"
	EventSessionEnd    = "session.end"
	EventSessionLoad   = "session.load"
	EventSessionResume = "session.resume"
	EventSessionSave   = "session.save"

	// Agent events
	EventAgentModeChange       = "agent.mode.change"
	EventAgentPermissionChange = "agent.permission.change"
	EventAgentTierChange       = "agent.tier.change"
	EventAgentQueryStart       = "agent.query.start"
	EventAgentQueryComplete    = "agent.query.complete"

	// Context events
	EventContextAdd     = "context.add"
	EventContextCompact = "context.compact"
	EventContextWarning = "context.warning"
	EventContextClear   = "context.clear"

	// LLM events
	EventLLMRequest      = "llm.request"
	EventLLMResponse     = "llm.response"
	EventLLMError        = "llm.error"
	EventLLMStreamStart  = "llm.stream.start"
	EventLLMStreamChunk  = "llm.stream.chunk"
	EventLLMStreamEnd    = "llm.stream.end"

	// Tool events
	EventToolStart    = "tool.start"
	EventToolComplete = "tool.complete"
	EventToolDenied   = "tool.denied"
	EventToolError    = "tool.error"

	// Memory events
	EventMemoryStore  = "memory.store"
	EventMemoryRecall = "memory.recall"
	EventMemoryForget = "memory.forget"

	// Plan events
	EventPlanCreate   = "plan.create"
	EventPlanStepStart    = "plan.step.start"
	EventPlanStepComplete = "plan.step.complete"

	// Intent classification events
	EventIntentClassify = "intent.classify"

	// Error events
	EventError = "error"
)
