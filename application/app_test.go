package application_test

import (
	"errors"

	"github.com/cloudfoundry/bosh-bootloader/application"
	"github.com/cloudfoundry/bosh-bootloader/fakes"
	"github.com/cloudfoundry/bosh-bootloader/storage"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

type setNewKeyPairName struct{}

func (snkp setNewKeyPairName) Execute(subcommandFlags []string, state storage.State) error {
	state.KeyPair = storage.KeyPair{
		Name:       "some-new-keypair-name",
		PublicKey:  state.KeyPair.PublicKey,
		PrivateKey: state.KeyPair.PrivateKey,
	}

	return nil
}

func (snkp setNewKeyPairName) Usage() string { return "" }

var _ = Describe("App", func() {
	var (
		app        application.App
		helpCmd    *fakes.Command
		versionCmd *fakes.Command
		someCmd    *fakes.Command
		errorCmd   *fakes.Command
		usage      *fakes.Usage
		stateStore *fakes.StateStore
	)

	var NewAppWithConfiguration = func(configuration application.Configuration) application.App {
		return application.New(application.CommandSet{
			"help":                 helpCmd,
			"version":              versionCmd,
			"some":                 someCmd,
			"error":                errorCmd,
			"set-new-keypair-name": setNewKeyPairName{},
		},
			configuration,
			stateStore,
			usage,
		)
	}

	BeforeEach(func() {
		helpCmd = &fakes.Command{}
		versionCmd = &fakes.Command{}
		errorCmd = &fakes.Command{}

		someCmd = &fakes.Command{}
		someCmd.ExecuteCall.PassState = true

		usage = &fakes.Usage{}
		stateStore = &fakes.StateStore{}

		app = NewAppWithConfiguration(application.Configuration{})
	})

	Describe("Run", func() {
		Context("executing commands", func() {
			It("executes the command with flags", func() {
				app = NewAppWithConfiguration(application.Configuration{
					Command: "some",
					SubcommandFlags: []string{
						"--first-subcommand-flag", "first-value",
						"--second-subcommand-flag", "second-value",
					},
					Global: application.GlobalConfiguration{
						StateDir:         "some/state/dir",
						EndpointOverride: "some-endpoint-override",
					},
					State: storage.State{
						AWS: storage.AWS{
							AccessKeyID:     "some-access-key-id",
							SecretAccessKey: "some-secret-access-key",
							Region:          "some-region",
						},
					},
				})

				Expect(app.Run()).To(Succeed())

				Expect(someCmd.ExecuteCall.CallCount).To(Equal(1))
				Expect(someCmd.ExecuteCall.Receives.SubcommandFlags).To(Equal([]string{
					"--first-subcommand-flag", "first-value",
					"--second-subcommand-flag", "second-value",
				}))
			})
		})

		Context("when subcommand flags contains help", func() {
			DescribeTable("prints command specific usage when help subcommand flag is provided", func(helpFlag string) {
				someCmd.UsageCall.Returns.Usage = "some usage message"

				app = NewAppWithConfiguration(application.Configuration{
					Command:         "some",
					SubcommandFlags: []string{helpFlag},
				})

				Expect(app.Run()).To(Succeed())
				Expect(someCmd.UsageCall.CallCount).To(Equal(1))
				Expect(usage.PrintCommandUsageCall.CallCount).To(Equal(1))
				Expect(usage.PrintCommandUsageCall.Receives.Message).To(Equal("some usage message"))
				Expect(usage.PrintCommandUsageCall.Receives.Command).To(Equal("some"))
				Expect(someCmd.ExecuteCall.CallCount).To(Equal(0))
			},
				Entry("when --help is provided", "--help"),
				Entry("when -h is provided", "-h"),
			)
		})

		Context("when help is called with a command", func() {
			It("prints the command specific help", func() {
				someCmd.UsageCall.Returns.Usage = "some usage message"

				app = NewAppWithConfiguration(application.Configuration{
					Command:         "help",
					SubcommandFlags: []string{"some"},
				})

				Expect(app.Run()).To(Succeed())
				Expect(someCmd.UsageCall.CallCount).To(Equal(1))
				Expect(usage.PrintCommandUsageCall.CallCount).To(Equal(1))
				Expect(usage.PrintCommandUsageCall.Receives.Message).To(Equal("some usage message"))
				Expect(usage.PrintCommandUsageCall.Receives.Command).To(Equal("some"))
				Expect(someCmd.ExecuteCall.CallCount).To(Equal(0))
			})

			Context("failure cases", func() {
				It("prints the usage when a invalid subcommand is passed", func() {
					app = NewAppWithConfiguration(application.Configuration{
						Command:         "help",
						SubcommandFlags: []string{"invalid-command"},
					})

					err := app.Run()
					Expect(err).To(MatchError("unknown command: invalid-command"))
					Expect(someCmd.ExecuteCall.CallCount).To(Equal(0))
					Expect(usage.PrintCall.CallCount).To(Equal(1))
				})
			})
		})

		Context("when subcommand flags contains version", func() {
			DescribeTable("prints version when version subcommand flag is provided", func(versionFlag string) {
				app = NewAppWithConfiguration(application.Configuration{
					Command:         "some",
					SubcommandFlags: []string{versionFlag},
				})

				Expect(app.Run()).To(Succeed())
				Expect(someCmd.ExecuteCall.CallCount).To(Equal(0))
				Expect(versionCmd.ExecuteCall.CallCount).To(Equal(1))
				Expect(versionCmd.ExecuteCall.Receives.SubcommandFlags).To(Equal([]string{}))
				Expect(versionCmd.ExecuteCall.Receives.State).To(Equal(storage.State{}))
			},
				Entry("when --version is provided", "--version"),
				Entry("when -v is provided", "-v"),
			)

			Context("error cases", func() {
				It("returns an error when version command is not part of the command set", func() {
					app = application.New(application.CommandSet{
						"some": someCmd,
					}, application.Configuration{
						Command:         "some",
						SubcommandFlags: []string{"-v"},
					}, storage.Store{}, usage)

					err := app.Run()
					Expect(err).To(MatchError("unknown command: version"))
				})
			})
		})

		Context("error cases", func() {
			Context("when an unknown command is provided", func() {
				It("prints usage and returns an error", func() {
					app = NewAppWithConfiguration(application.Configuration{
						Command: "some-unknown-command",
					})
					err := app.Run()
					Expect(err).To(MatchError("unknown command: some-unknown-command"))
					Expect(usage.PrintCall.CallCount).To(Equal(1))
				})
			})

			Context("when the command fails to execute", func() {
				It("returns an error", func() {
					errorCmd.ExecuteCall.Returns.Error = errors.New("error executing command")
					app = NewAppWithConfiguration(application.Configuration{
						Command: "error",
					})
					err := app.Run()
					Expect(err).To(MatchError("error executing command"))
				})
			})
		})
	})
})
