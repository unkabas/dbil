package postgres

// BlockedError is returned by Manager.Execute when the connection's tag
// policy disallows the statement outright (no confirmation can change it).
type BlockedError struct {
	Reason string
}

// Error implements error.
func (e *BlockedError) Error() string { return "blocked by policy: " + e.Reason }

// ConfirmationRequiredError is returned by Manager.Execute when the policy
// requires X-Confirm: yes for this class of statement but the caller did
// not supply it. The same statement re-sent with confirm=true would run.
type ConfirmationRequiredError struct {
	Reason string
}

// Error implements error.
func (e *ConfirmationRequiredError) Error() string { return "confirmation required: " + e.Reason }
