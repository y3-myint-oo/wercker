package main

import "github.com/codegangsta/cli"

// Flags for setting these options from the CLI
var (
	// These flags tell us where to go for operations
	endpointFlags = []cli.Flag{
		// deprecated
		cli.StringFlag{Name: "wercker-endpoint", Value: "", Usage: "Deprecated.", Hidden: true},
		cli.StringFlag{Name: "base-url", Value: "https://app.wercker.com", Usage: "Base url for the wercker app.", Hidden: true},
	}

	// These flags let us auth to wercker services
	authFlags = []cli.Flag{
		cli.StringFlag{Name: "auth-token", Usage: "Authentication token to use."},
		cli.StringFlag{Name: "auth-token-store", Value: "~/.wercker/token", Usage: "Where to store the token after a login.", Hidden: true},
	}
	dockerFlags = []cli.Flag{
		cli.StringFlag{Name: "docker-host", Value: "", Usage: "Docker api endpoint.", EnvVar: "DOCKER_HOST"},
		cli.StringFlag{Name: "docker-tls-verify", Value: "0", Usage: "Docker api tls verify.", EnvVar: "DOCKER_TLS_VERIFY"},
		cli.StringFlag{Name: "docker-cert-path", Value: "", Usage: "Docker api cert path.", EnvVar: "DOCKER_CERT_PATH"},
		cli.StringSliceFlag{Name: "docker-dns", Value: &cli.StringSlice{0: "8.8.8.8", 1: "8.8.4.4"}, Usage: "Docker DNS server.", EnvVar: "DOCKER_DNS", Hidden: true},
		cli.BoolFlag{Name: "docker-local", Usage: "Don't interact with remote repositories"},
	}

	// These flags control where we store local files
	localPathFlags = []cli.Flag{
		cli.StringFlag{Name: "working-dir", Value: "", Usage: "Path where we store working files."},

		// following -dir flags are DEPRECATED, here for BC
		cli.StringFlag{Name: "build-dir", Value: "./_builds", Usage: "Path where created builds live."},
		cli.StringFlag{Name: "cache-dir", Value: "./_cache", Usage: "Path for storing pipeline cache."},
		cli.StringFlag{Name: "container-dir", Value: "./_containers", Usage: "Path where exported containers live."},
		cli.StringFlag{Name: "project-dir", Value: "./_projects", Usage: "Path where downloaded projects live."},
		cli.StringFlag{Name: "step-dir", Value: "./_steps", Usage: "Path where downloaded steps live."},
	}

	// These flags control paths on the guest and probably shouldn't change
	internalPathFlags = []cli.Flag{
		cli.StringFlag{Name: "mnt-root", Value: "/mnt", Usage: "Directory on the guest where volumes are mounted.", Hidden: true},
		cli.StringFlag{Name: "guest-root", Value: "/pipeline", Usage: "Directory on the guest where work is done.", Hidden: true},
		cli.StringFlag{Name: "report-root", Value: "/report", Usage: "Directory on the guest where reports will be written.", Hidden: true},
	}

	// These flags are usually pulled from the env
	werckerFlags = []cli.Flag{
		cli.StringFlag{Name: "build-id", Value: "", EnvVar: "WERCKER_BUILD_ID", Hidden: true,
			Usage: "The build id."},
		cli.StringFlag{Name: "deploy-id", Value: "", EnvVar: "WERCKER_DEPLOY_ID", Hidden: true,
			Usage: "The deploy id."},
		cli.StringFlag{Name: "deploy-target", Value: "", EnvVar: "WERCKER_DEPLOYTARGET_NAME",
			Usage: "The deploy target name."},
		cli.StringFlag{Name: "application-id", Value: "", EnvVar: "WERCKER_APPLICATION_ID", Hidden: true,
			Usage: "The application id."},
		cli.StringFlag{Name: "application-name", Value: "", EnvVar: "WERCKER_APPLICATION_NAME", Hidden: true,
			Usage: "The application name."},
		cli.StringFlag{Name: "application-owner-name", Value: "", EnvVar: "WERCKER_APPLICATION_OWNER_NAME", Hidden: true,
			Usage: "The application owner name."},
		cli.StringFlag{Name: "application-started-by-name", Value: "", EnvVar: "WERCKER_APPLICATION_STARTED_BY_NAME", Hidden: true,
			Usage: "The name of the user who started the application."},
		cli.StringFlag{Name: "pipeline", Value: "", EnvVar: "WERCKER_PIPELINE", Hidden: true,
			Usage: "Alternate pipeline name to execute."},
	}

	gitFlags = []cli.Flag{
		cli.StringFlag{Name: "git-domain", Value: "", Usage: "Git domain.", EnvVar: "WERCKER_GIT_DOMAIN", Hidden: true},
		cli.StringFlag{Name: "git-owner", Value: "", Usage: "Git owner.", EnvVar: "WERCKER_GIT_OWNER", Hidden: true},
		cli.StringFlag{Name: "git-repository", Value: "", Usage: "Git repository.", EnvVar: "WERCKER_GIT_REPOSITORY", Hidden: true},
		cli.StringFlag{Name: "git-branch", Value: "", Usage: "Git branch.", EnvVar: "WERCKER_GIT_BRANCH", Hidden: true},
		cli.StringFlag{Name: "git-commit", Value: "", Usage: "Git commit.", EnvVar: "WERCKER_GIT_COMMIT", Hidden: true},
	}

	// These flags affect our registry interactions
	registryFlags = []cli.Flag{
		cli.StringFlag{Name: "commit", Value: "", Usage: "Commit the build result locally."},
		cli.StringFlag{Name: "tag", Value: "", Usage: "Tag for this build.", EnvVar: "WERCKER_GIT_BRANCH"},
		cli.StringFlag{Name: "message", Value: "", Usage: "Message for this build."},
	}

	// These flags affect our artifact interactions
	artifactFlags = []cli.Flag{
		cli.BoolFlag{Name: "artifacts", Usage: "Store artifacts."},
		cli.BoolFlag{Name: "no-remove", Usage: "Don't remove the containers."},
		cli.BoolFlag{Name: "store-local", Usage: "Store artifacts and containers locally."},
		cli.BoolFlag{Name: "store-s3",
			Usage: `Store artifacts and containers on s3.
			This requires access to aws credentials, pulled from any of the usual places
			(~/.aws/config, AWS_SECRET_ACCESS_KEY, etc), or from the --aws-secret-key and
			--aws-access-key flags. It will upload to a bucket defined by --s3-bucket in
			the region named by --aws-region`},
	}

	// These flags affect our local execution environment
	devFlags = []cli.Flag{
		cli.StringFlag{Name: "environment", Value: "ENVIRONMENT", Usage: "Specify additional environment variables in a file."},
		cli.BoolFlag{Name: "verbose", Usage: "Print more information."},
		cli.BoolFlag{Name: "no-colors", Usage: "Wercker output will not use colors (does not apply to step output)."},
		cli.BoolFlag{Name: "debug", Usage: "Print additional debug information."},
		cli.BoolFlag{Name: "journal", Usage: "Send logs to systemd-journald. Suppresses stdout logging."},
	}

	// These flags are advanced dev settings
	internalDevFlags = []cli.Flag{
		cli.BoolTFlag{Name: "direct-mount", Usage: "Mount our binds read-write to the pipeline path."},
		cli.StringSliceFlag{Name: "publish", Value: &cli.StringSlice{}, Usage: "Publish a port from the main container, same format as docker --publish."},
		cli.BoolFlag{Name: "attach-on-error", Usage: "Attach shell to container if a step fails.", Hidden: true},
		cli.BoolTFlag{Name: "enable-dev-steps", Hidden: true, Usage: `
		Enable internal dev steps.
		This enables:
		- internal/watch
		`},
	}

	// These flags are advanced build settings
	internalBuildFlags = []cli.Flag{
		cli.BoolFlag{Name: "direct-mount", Usage: "Mount our binds read-write to the pipeline path."},
		cli.StringSliceFlag{Name: "publish", Value: &cli.StringSlice{}, Usage: "Publish a port from the main container, same format as docker --publish."},
		cli.BoolFlag{Name: "attach-on-error", Usage: "Attach shell to container if a step fails.", Hidden: true},
		cli.BoolFlag{Name: "enable-dev-steps", Hidden: true, Usage: `
		Enable internal dev steps.
		This enables:
		- internal/watch
		`},
	}

	// AWS bits
	awsFlags = []cli.Flag{
		cli.StringFlag{Name: "aws-secret-key", Value: "", Usage: "Secret access key. Used for artifact storage."},
		cli.StringFlag{Name: "aws-access-key", Value: "", Usage: "Access key id. Used for artifact storage."},
		cli.StringFlag{Name: "s3-bucket", Value: "wercker-development", Usage: "Bucket for artifact storage."},
		cli.StringFlag{Name: "aws-region", Value: "us-east-1", Usage: "AWS region to use for artifact storage."},
	}

	// keen.io bits
	keenFlags = []cli.Flag{
		cli.BoolFlag{Name: "keen-metrics", Usage: "Report metrics to keen.io.", Hidden: true},
		cli.StringFlag{Name: "keen-project-write-key", Value: "", Usage: "Keen write key.", Hidden: true},
		cli.StringFlag{Name: "keen-project-id", Value: "", Usage: "Keen project id.", Hidden: true},
	}

	// Wercker Reporter settings
	reporterFlags = []cli.Flag{
		cli.BoolFlag{Name: "report", Usage: "Report logs back to wercker (requires build-id, wercker-host, wercker-token).", Hidden: true},
		cli.StringFlag{Name: "wercker-host", Usage: "Wercker host to use for wercker reporter.", Hidden: true},
		cli.StringFlag{Name: "wercker-token", Usage: "Wercker token to use for wercker reporter.", Hidden: true},
	}

	// These options might be overwritten by the wercker.yml
	configFlags = []cli.Flag{
		cli.StringFlag{Name: "source-dir", Value: "", Usage: "Source path relative to checkout root."},
		cli.Float64Flag{Name: "no-response-timeout", Value: 5, Usage: "Timeout if no script output is received in this many minutes."},
		cli.Float64Flag{Name: "command-timeout", Value: 25, Usage: "Timeout if command does not complete in this many minutes."},
		cli.StringFlag{Name: "wercker-yml", Value: "", Usage: "Specify a specific yaml file."},
	}

	pullFlags = [][]cli.Flag{
		[]cli.Flag{
			cli.StringFlag{Name: "branch", Value: "", Usage: "Filter on this branch."},
			cli.StringFlag{Name: "result", Value: "", Usage: "Filter on this result (passed or failed)."},
			cli.StringFlag{Name: "output", Value: "./repository.tar", Usage: "Path to repository."},
			cli.BoolFlag{Name: "load", Usage: "Load the container into docker after downloading."},
			cli.BoolFlag{Name: "f, force", Usage: "Override output if it already exists."},
		},
	}

	GlobalFlags = [][]cli.Flag{
		devFlags,
		endpointFlags,
		authFlags,
	}

	DockerFlags = [][]cli.Flag{
		dockerFlags,
	}

	PipelineFlags = [][]cli.Flag{
		localPathFlags,
		werckerFlags,
		dockerFlags,
		internalBuildFlags,
		gitFlags,
		registryFlags,
		artifactFlags,
		awsFlags,
		configFlags,
	}

	DevPipelineFlags = [][]cli.Flag{
		localPathFlags,
		werckerFlags,
		dockerFlags,
		internalDevFlags,
		gitFlags,
		registryFlags,
		artifactFlags,
		awsFlags,
		configFlags,
	}

	WerckerInternalFlags = [][]cli.Flag{
		internalPathFlags,
		keenFlags,
		reporterFlags,
	}
)

func flagsFor(flagSets ...[][]cli.Flag) []cli.Flag {
	all := []cli.Flag{}
	for _, flagSet := range flagSets {
		for _, x := range flagSet {
			all = append(all, x...)
		}
	}
	return all
}
