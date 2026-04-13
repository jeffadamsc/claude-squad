package tasks

func init() {
	Register(&EditAddLog{})
	Register(&EditAddComment{})
}

// EditAddLog asks Claude to add a log statement.
type EditAddLog struct{}

func (t *EditAddLog) Name() string     { return "edit-add-log" }
func (t *EditAddLog) Category() string { return "edit" }
func (t *EditAddLog) Prompt() string {
	return "Add a debug log statement at the start of the Validate function on the Session model in verve-backend/models/session.go. The log should print 'Validating session' followed by the session ID."
}
func (t *EditAddLog) Validate(output string) error { return nil }

// EditAddComment asks Claude to add a comment.
type EditAddComment struct{}

func (t *EditAddComment) Name() string     { return "edit-add-comment" }
func (t *EditAddComment) Category() string { return "edit" }
func (t *EditAddComment) Prompt() string {
	return "Add a comment explaining the purpose of the TableName function on the Session model in verve-backend/models/session.go."
}
func (t *EditAddComment) Validate(output string) error { return nil }
