//go:build gui

// Package gui implements the experimental GTK 4 frontend for e9s.
package gui

import (
	"context"
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
)

const applicationID = "com.github.dostrow.e9s.gui"

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
	Config          *config.Config
	ReloadConfig    func() config.Config
	DefaultCluster  string
	Profile         string
	Region          string
	RefreshInterval int
}

// Run starts the experimental GTK application.
func Run(options Options) error {
	app := gtk.NewApplication(applicationID, gio.ApplicationFlagsNone)
	ctx, cancel := context.WithCancel(context.Background())
	var window *mainWindow

	app.ConnectActivate(func() {
		if window != nil {
			window.window.Present()
			return
		}
		initializeSourceEditor()
		installStyles()
		window = newMainWindow(ctx, app, options)
		installSemanticStyles(window)
		window.window.Present()
		window.start()
	})
	app.ConnectShutdown(cancel)

	if code := app.Run(nil); code != 0 {
		return &ExitError{Code: code}
	}
	return nil
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
		gtk.STYLE_PROVIDER_PRIORITY_APPLICATION,
	)
}
