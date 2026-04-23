package ui

// KeyBindings holds all configurable key bindings. Each field is the key string
// that msg.String() returns (e.g. "ctrl+s", "e", "`", "W").
// Defaults are set in NewKeyBindings, then overridden from config.
type KeyBindings struct {
	// Global
	SwitchMode   string
	ReopenPicker string
	PauseResume  string
	EditConfig   string
	SwitchRegion string

	// ECS
	ForceRedeploy   string
	Scale           string
	ServiceDetail   string
	ServiceLogs     string
	Metrics         string
	StandaloneTasks string
	TaskLogs        string
	StopTask        string
	ECSExec         string
	EnvVars         string
	ToggleScaleIn   string

	// Log viewer
	LogFollow     string
	LogTimestamp  string
	LogOlder      string
	LogNewer      string
	LogCopy       string
	LogOpenEditor string
	LogSave       string
	LogCorrelate  string

	// CloudWatch Logs
	TailStream    string
	TailGroup     string
	BrowseStreams string
	SavePath      string

	// CloudWatch Alarms
	ToggleActions string
	SetAlarmState string

	// SSM / SM
	EditValue   string
	CloneSecret string
	CopyARN     string

	// S3
	Download string

	// Lambda
	EditCode string

	// DynamoDB
	FilterScan string
	PartiQL    string
	NextPage   string
	EditField  string
	CloneItem  string

	// SQS
	Messages     string
	PollMessages string
	SendMessage  string
	CloneSend    string
	DeleteMsg    string
	NavigateDLQ  string

	// CodeBuild
	StartBuild string
	StopBuild  string
	ViewLogs   string

	// EC2
	SSMSession     string
	ConsoleOutput  string
	StartInstance  string
	StopInstance   string
	RebootInstance string
	TermInstance   string

	// Route53
	TestDNS      string
	EditRecord   string
	DeleteRecord string
	NewRecord    string

	// ECR
	StartScan   string
	CopyURI     string
	DeleteImage string

	// OpenTofu
	RunPlan  string
	RunApply string
	RunInit  string

	// Shared
	Save      string // W — save/bookmark
	Search    string // s — general search/find
	Timestamp string // t — toggle timestamps
}

// NewKeyBindings returns KeyBindings with all defaults.
func NewKeyBindings() KeyBindings {
	return KeyBindings{
		// Global
		SwitchMode:   "`",
		ReopenPicker: "ctrl+p",
		PauseResume:  "ctrl+s",
		EditConfig:   "ctrl+e",
		SwitchRegion: "ctrl+r",

		// ECS
		ForceRedeploy:   "r",
		Scale:           "s",
		ServiceDetail:   "d",
		ServiceLogs:     "L",
		Metrics:         "m",
		StandaloneTasks: "S",
		TaskLogs:        "l",
		StopTask:        "x",
		ECSExec:         "e",
		EnvVars:         "E",
		ToggleScaleIn:   "I",

		// Log viewer
		LogFollow:     "f",
		LogTimestamp:  "t",
		LogOlder:      "[",
		LogNewer:      "]",
		LogCopy:       "y",
		LogOpenEditor: "o",
		LogSave:       "w",
		LogCorrelate:  "c",

		// CW Logs
		TailStream:    "l",
		TailGroup:     "L",
		BrowseStreams: "L",
		SavePath:      "W",

		// CW Alarms
		ToggleActions: "a",
		SetAlarmState: "S",

		// SSM / SM
		EditValue:   "e",
		CloneSecret: "c",
		CopyARN:     "y",

		// S3
		Download: "D",

		// Lambda
		EditCode: "c",

		// DynamoDB
		FilterScan: "f",
		PartiQL:    "p",
		NextPage:   "]",
		EditField:  "e",
		CloneItem:  "c",

		// SQS
		Messages:     "m",
		PollMessages: "p",
		SendMessage:  "s",
		CloneSend:    "c",
		DeleteMsg:    "x",
		NavigateDLQ:  "n",

		// CodeBuild
		StartBuild: "b",
		StopBuild:  "x",
		ViewLogs:   "l",

		// EC2
		SSMSession:     "e",
		ConsoleOutput:  "c",
		StartInstance:  "S",
		StopInstance:   "X",
		RebootInstance: "r",
		TermInstance:   "T",

		// Route53
		TestDNS:      "t",
		EditRecord:   "e",
		DeleteRecord: "x",
		NewRecord:    "n",

		// ECR
		StartScan:   "s",
		CopyURI:     "y",
		DeleteImage: "x",

		// OpenTofu
		RunPlan:  "p",
		RunApply: "a",
		RunInit:  "i",

		// Shared
		Save:      "W",
		Search:    "s",
		Timestamp: "t",
	}
}

// ApplyOverrides applies user-configured key overrides from the config map.
func (kb *KeyBindings) ApplyOverrides(overrides map[string]string) {
	if len(overrides) == 0 {
		return
	}
	for action, key := range overrides {
		switch action {
		// Global
		case "switch_mode":
			kb.SwitchMode = key
		case "reopen_picker":
			kb.ReopenPicker = key
		case "pause_resume":
			kb.PauseResume = key
		case "edit_config":
			kb.EditConfig = key
		case "switch_region":
			kb.SwitchRegion = key

		// ECS
		case "force_redeploy":
			kb.ForceRedeploy = key
		case "scale":
			kb.Scale = key
		case "service_detail":
			kb.ServiceDetail = key
		case "service_logs":
			kb.ServiceLogs = key
		case "metrics":
			kb.Metrics = key
		case "standalone_tasks":
			kb.StandaloneTasks = key
		case "task_logs":
			kb.TaskLogs = key
		case "stop_task":
			kb.StopTask = key
		case "ecs_exec":
			kb.ECSExec = key
		case "env_vars":
			kb.EnvVars = key
		case "toggle_scale_in":
			kb.ToggleScaleIn = key

		// Log viewer
		case "log_follow":
			kb.LogFollow = key
		case "log_timestamp":
			kb.LogTimestamp = key
		case "log_older":
			kb.LogOlder = key
		case "log_newer":
			kb.LogNewer = key
		case "log_copy":
			kb.LogCopy = key
		case "log_open_editor":
			kb.LogOpenEditor = key
		case "log_save":
			kb.LogSave = key
		case "log_correlate":
			kb.LogCorrelate = key

		// CW
		case "tail_stream":
			kb.TailStream = key
		case "tail_group":
			kb.TailGroup = key
		case "search_logs":
			kb.Search = key
		case "browse_streams":
			kb.BrowseStreams = key
		case "save_path":
			kb.SavePath = key

		// Alarms
		case "toggle_actions":
			kb.ToggleActions = key
		case "set_alarm_state":
			kb.SetAlarmState = key

		// SSM / SM
		case "edit_value":
			kb.EditValue = key
		case "clone_secret":
			kb.CloneSecret = key
		case "copy_arn":
			kb.CopyARN = key

		// S3
		case "download":
			kb.Download = key

		// Lambda
		case "edit_code":
			kb.EditCode = key

		// DynamoDB
		case "filter_scan":
			kb.FilterScan = key
		case "partiql":
			kb.PartiQL = key
		case "next_page":
			kb.NextPage = key
		case "edit_field":
			kb.EditField = key
		case "clone_item":
			kb.CloneItem = key

		// SQS
		case "messages":
			kb.Messages = key
		case "poll_messages":
			kb.PollMessages = key
		case "send_message":
			kb.SendMessage = key
		case "clone_send":
			kb.CloneSend = key
		case "delete_msg":
			kb.DeleteMsg = key
		case "navigate_dlq":
			kb.NavigateDLQ = key

		// CodeBuild
		case "start_build":
			kb.StartBuild = key
		case "stop_build":
			kb.StopBuild = key
		case "view_logs":
			kb.ViewLogs = key
		case "search_build_logs":
			kb.Search = key

		// EC2
		case "ssm_session":
			kb.SSMSession = key
		case "console_output":
			kb.ConsoleOutput = key
		case "start_instance":
			kb.StartInstance = key
		case "stop_instance":
			kb.StopInstance = key
		case "reboot_instance":
			kb.RebootInstance = key
		case "terminate_instance":
			kb.TermInstance = key

		// Route53
		case "test_dns":
			kb.TestDNS = key
		case "edit_record":
			kb.EditRecord = key
		case "delete_record":
			kb.DeleteRecord = key
		case "new_record":
			kb.NewRecord = key

		// ECR
		case "start_scan":
			kb.StartScan = key
		case "copy_uri":
			kb.CopyURI = key
		case "delete_image":
			kb.DeleteImage = key

		// OpenTofu
		case "run_plan":
			kb.RunPlan = key
		case "run_apply":
			kb.RunApply = key
		case "run_init":
			kb.RunInit = key

		// Shared
		case "save":
			kb.Save = key
		case "search":
			kb.Search = key
		case "timestamp":
			kb.Timestamp = key
		}
	}
}
