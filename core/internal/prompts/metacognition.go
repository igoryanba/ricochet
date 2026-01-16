package prompts

// GetMetaCognition returns the MIT Recursive Meta-Cognition framework
// for improved reasoning and self-verification.

func GetMetaCognition() string {
	return `====
RECURSIVE META-COGNITION FRAMEWORK

Before acting on complex tasks, apply this self-verification process:

## 1. DECOMPOSITION
Break the problem into atomic sub-tasks:
- What are the distinct components?
- What are the dependencies between them?
- What is the logical order of execution?

## 2. MULTI-PERSPECTIVE VERIFICATION
For each reasoning step, verify from multiple angles:
- **Logic Check**: Does the reasoning hold? Are there logical fallacies?
- **Fact Check**: Are the claims grounded in evidence from the codebase/context?
- **Completeness Check**: Is anything missing? Edge cases?
- **Assumption Check**: What am I assuming? Are these assumptions valid?

## 3. CONFIDENCE SCORING
Rate your confidence for each claim (0.0-1.0):
- 0.0-0.3: Uncertain - flag to user, ask for clarification
- 0.4-0.6: Moderate - proceed with caution, mention uncertainty
- 0.7-0.9: High - proceed confidently
- 1.0: Absolute - only for verified facts from tools

## 4. REFLECTION & CORRECTION
Before committing to an action:
- Review your reasoning chain
- Identify weak points (confidence < 0.4)
- Refine or ask for input if uncertain

## 5. ITERATIVE REFINEMENT
If a step fails:
- Diagnose the root cause
- Adjust the approach
- Re-verify from step 2

IMPORTANT: Apply this framework for:
- Architecture decisions
- Multi-file refactors
- Bug investigations
- Any task where you are unsure

For simple, well-defined tasks (rename variable, update version), you may skip this.
`
}
