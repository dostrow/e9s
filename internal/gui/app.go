//go:build gui

// Package gui implements the experimental GTK 4 frontend for e9s.
package gui

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	e9saws "github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/tofu"
)

// applicationID is the stable reverse-DNS identity shared by the GTK
// application, desktop entry, icon, AppStream metadata, and portable bundles.
// Keep user configuration under the existing e9s XDG directory; this identity
// is for desktop integration rather than configuration storage.
const applicationID = "io.github.dostrow.e9s"

type ECSService interface {
	ListClusters(context.Context) ([]model.Cluster, error)
	ListServices(context.Context, string) ([]model.Service, error)
	ListTasks(context.Context, string, string) ([]model.Task, error)
	ListStandaloneTasks(context.Context, string) ([]model.Task, error)
	ListStoppedStandaloneTasks(context.Context, string, string, int) (model.TaskPage, error)
	ListStoppedServiceTasks(context.Context, string, string, string, int) (model.TaskPage, error)
	ForceDeployment(context.Context, string, string) error
	ScaleService(context.Context, string, string, int) error
	StopTask(context.Context, string, string, string) error
	RunTask(context.Context, model.RunTaskRequest) ([]model.Task, error)
	GetServiceMetrics(context.Context, string, string, time.Duration) (*model.ServiceMetrics, error)
	GetTaskMetrics(context.Context, string, string, model.Task, time.Duration) (*model.ServiceMetrics, error)
	ListServiceAlarms(context.Context, string, string) ([]model.AlarmState, error)
	ScaleInSuspended(context.Context, string, string) (bool, error)
	SetScaleInSuspended(context.Context, string, string, bool) error
	ListTaskDefinitions(context.Context, string) ([]model.TaskDefRef, error)
	GetTaskDefinition(context.Context, string) (*model.TaskDefSummary, error)
	TaskDefinitionDiff(context.Context, string, string) (string, error)
	TaskDefinitionEditorDocument(string) (string, error)
	RegisterTaskDefinitionJSON(context.Context, string) (*model.TaskDefSummary, error)
	TaskDefinitionEnvironment(context.Context, string, string, bool) ([]model.EnvVar, error)
	PrepareExecSession(context.Context, string, model.Task, string, string) (model.ExecLaunch, error)
	ContainerLogSource(context.Context, model.Task, string) (model.LogSource, error)
	ServiceLogSource(context.Context, string, string) (model.LogSource, error)
}

type LogService interface {
	ListGroups(context.Context, string) ([]model.LogGroup, error)
	ListStreams(context.Context, string, string) ([]model.LogStream, error)
	Fetch(context.Context, string, model.LogQuery) (model.LogPage, error)
}

type AlarmService interface {
	List(context.Context, string) ([]model.Alarm, error)
	Detail(context.Context, string) (*model.AlarmDetail, error)
	SetActionsEnabled(context.Context, string, bool) error
	SetState(context.Context, string, string, string) error
}

type SSMService interface {
	List(context.Context, string) ([]model.Parameter, error)
	Detail(context.Context, string) (*model.Parameter, error)
	Update(context.Context, string, string) error
}

type SecretsService interface {
	List(context.Context, string) ([]model.Secret, error)
	Detail(context.Context, string) (*model.SecretValue, error)
	Create(context.Context, string, string, string) error
	Update(context.Context, string, string) error
}

type LambdaService interface {
	List(context.Context, string) ([]model.LambdaFunction, error)
	Detail(context.Context, string) (*model.LambdaFunction, error)
	Environment(context.Context, string, bool) ([]model.EnvVar, error)
	PackageType(context.Context, string) (string, error)
	DownloadCode(context.Context, string) (string, error)
	UpdateCode(context.Context, string, []byte) error
}

type CodeBuildService interface {
	ListProjects(context.Context, string) ([]model.CodeBuildProject, error)
	ListBuilds(context.Context, string, int) ([]model.CodeBuildBuild, error)
	Detail(context.Context, string) (*model.CodeBuildDetail, error)
	Start(context.Context, string, string) (*model.CodeBuildBuild, error)
	Stop(context.Context, string, string) error
}

type EC2Service interface {
	List(context.Context, string) ([]model.EC2Instance, error)
	Detail(context.Context, string) (*model.EC2InstanceDetail, error)
	ConsoleOutput(context.Context, string) (string, error)
	PrepareSession(context.Context, string, string) (model.ExecLaunch, error)
	Start(context.Context, string, string) error
	Stop(context.Context, string, string) error
	Reboot(context.Context, string, string) error
	Terminate(context.Context, string, string) error
	Metrics(context.Context, string, time.Duration) (*model.MetricSnapshot, error)
}

type EC2NetworkService interface {
	SecurityGroups(context.Context, string, string) ([]model.EC2SecurityGroup, error)
	SecurityGroup(context.Context, string) (*model.EC2SecurityGroup, error)
	VPCs(context.Context, string) ([]model.EC2VPC, error)
	VPC(context.Context, string) (*model.EC2VPC, error)
	Subnets(context.Context, string, string) ([]model.EC2Subnet, error)
	Subnet(context.Context, string) (*model.EC2Subnet, error)
}

type EBSService interface {
	List(context.Context, string) ([]model.EC2Volume, error)
	Detail(context.Context, string) (*model.EC2Volume, error)
}

type LoadBalancingService interface {
	List(context.Context, string) ([]model.EC2LoadBalancer, error)
	Detail(context.Context, string) (*model.EC2LoadBalancer, error)
	ListTargetGroups(context.Context, string) ([]model.EC2TargetGroup, error)
	TargetGroup(context.Context, string) (*model.EC2TargetGroup, error)
}

type ECRService interface {
	ListRepositories(context.Context, string) ([]model.ECRRepo, error)
	ListImages(context.Context, string) ([]model.ECRImage, error)
	Findings(context.Context, string, string) ([]model.ECRFinding, error)
	ScanFindings(context.Context, string, string) (model.ECRScan, error)
	StartScan(context.Context, string, model.ECRImage) error
	DeleteImage(context.Context, string, string) error
	ImageURI(string, model.ECRImage) (string, error)
}

type RDSService interface {
	Clusters(context.Context, string) ([]model.RDSCluster, error)
	Cluster(context.Context, string) (*model.RDSCluster, error)
	ClusterInstances(context.Context, string) ([]model.RDSInstance, error)
	List(context.Context, string) ([]model.RDSInstance, error)
	Detail(context.Context, string) (*model.RDSInstanceDetail, error)
	Metrics(context.Context, string, time.Duration) (*model.MetricSnapshot, error)
	ClusterMetrics(context.Context, string, time.Duration) (*model.MetricSnapshot, error)
}

type S3Service interface {
	Buckets(context.Context, string) ([]model.S3Bucket, error)
	Objects(context.Context, string, string) ([]model.S3Object, error)
	Search(context.Context, string, string) ([]model.S3Object, error)
	Detail(context.Context, string, string) (*model.S3ObjectDetail, error)
	Download(context.Context, model.S3DownloadRequest) (model.S3DownloadResult, error)
	DownloadWithProgress(context.Context, model.S3DownloadRequest, func(model.S3DownloadProgress)) (model.S3DownloadResult, error)
}

type DynamoDBService interface {
	Tables(context.Context, string) ([]string, error)
	Table(context.Context, string) (*model.DynamoTable, error)
	Scan(context.Context, model.DynamoScanRequest) (*model.DynamoPage, error)
	PartiQL(context.Context, string) ([]model.DynamoItem, error)
	Item(context.Context, string, []string, model.DynamoItem) (*model.DynamoItem, error)
	UpdateField(context.Context, model.DynamoFieldUpdate) error
	PutItem(context.Context, model.DynamoPutRequest) error
}

type SQSService interface {
	Queues(context.Context, string) ([]model.SQSQueue, error)
	Queue(context.Context, string) (*model.SQSQueueStats, error)
	Messages(context.Context, model.SQSReceiveRequest) ([]model.SQSMessage, error)
	ResolveQueueURL(context.Context, string) (string, error)
	DeleteMessage(context.Context, string, string) error
	SendMessage(context.Context, model.SQSSendRequest) (string, error)
}

type Route53Service interface {
	Zones(context.Context, string) ([]model.Route53Zone, error)
	Records(context.Context, string) ([]model.Route53Record, error)
	TestDNS(context.Context, string, model.Route53Record) (*model.Route53DNSAnswer, error)
	Create(context.Context, string, model.Route53Record) error
	Update(context.Context, string, model.Route53Record) error
	Delete(context.Context, string, model.Route53Record) error
}

type TofuService interface {
	Workspace(context.Context, string) (tofu.Workspace, error)
	Variables(context.Context, string) (tofu.VariablesDocument, error)
	SaveVariables(context.Context, tofu.VariablesDocument, string) (tofu.VariablesDocument, error)
	Resources(context.Context, string) ([]tofu.Resource, error)
	State(context.Context, string, string) (string, error)
	Plan(context.Context, string) (*tofu.PlanResult, string, error)
	Init(context.Context, string) (string, error)
	Apply(context.Context, string, string) (string, error)
	InitCommand(string) (tofu.Command, error)
	ApplyCommand(string, string) (tofu.Command, error)
	CleanupPlan(string)
}

type CostExplorerService interface {
	Report(context.Context, model.CostQuery, bool, bool, bool) (model.CostReport, model.CostCacheStatus, error)
	Anomalies(context.Context, time.Time, time.Time, bool) (model.CostAnomalyReport, model.CostCacheStatus, error)
}

type Options struct {
	ECS             ECSService
	Logs            LogService
	Alarms          AlarmService
	SSM             SSMService
	Secrets         SecretsService
	Lambda          LambdaService
	CodeBuild       CodeBuildService
	EC2             EC2Service
	EC2Network      EC2NetworkService
	EBS             EBSService
	LoadBalancing   LoadBalancingService
	ECR             ECRService
	RDS             RDSService
	S3              S3Service
	DynamoDB        DynamoDBService
	SQS             SQSService
	Route53         Route53Service
	Tofu            TofuService
	CostExplorer    CostExplorerService
	Config          *config.Config
	ReloadConfig    func() config.Config
	DefaultCluster  string
	Profile         string
	Region          string
	RefreshInterval int
	RequestSnapshot func() e9saws.RequestSnapshot
}

// Run starts the experimental GTK application.
func Run(options Options) error {
	configureHyprlandRenderer()
	app := gtk.NewApplication(applicationID, gio.ApplicationFlagsNone)
	gtk.WindowSetDefaultIconName(applicationID)
	ctx, cancel := context.WithCancel(context.Background())
	var window *mainWindow

	app.ConnectActivate(func() {
		if window != nil {
			window.window.Present()
			return
		}
		_, fontError := registerBundledFonts()
		initializeSourceEditor()
		installStyles()
		window = newMainWindow(ctx, app, options)
		installSemanticStyles(window)
		if fontError != nil {
			window.setStatus(fontError.Error(), true)
		}
		window.window.Present()
		window.start()
	})
	app.ConnectShutdown(cancel)

	if code := app.Run(nil); code != 0 {
		return &ExitError{Code: code}
	}
	return nil
}

// configureHyprlandRenderer avoids GTK's Vulkan swapchain path on native
// Hyprland/Wayland sessions. A renderer selected by the user always wins; the
// fallback is deliberately scoped so other compositors retain GTK's default.
// This must run before GTK creates the default display.
func configureHyprlandRenderer() bool {
	if strings.TrimSpace(os.Getenv("GSK_RENDERER")) != "" {
		return false
	}
	if strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) == "" {
		return false
	}
	backend := strings.ToLower(strings.TrimSpace(os.Getenv("GDK_BACKEND")))
	if backend != "" && !strings.HasPrefix(backend, "wayland") {
		return false
	}
	desktop := strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP") + ";" + os.Getenv("XDG_SESSION_DESKTOP"))
	if !strings.Contains(desktop, "hyprland") {
		return false
	}
	return os.Setenv("GSK_RENDERER", "gl") == nil
}

type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return "GTK application exited unsuccessfully"
}

func installStyles() {
	provider := gtk.NewCSSProvider()
	provider.LoadFromString(styleCSS)
	gtk.StyleContextAddProviderForDisplay(
		gdk.DisplayGetDefault(),
		provider,
		e9sStylePriority,
	)
}
