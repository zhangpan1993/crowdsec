package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http/httputil"
	"net/url"

	"github.com/crowdsecurity/crowdsec/pkg/apiclient"
	"github.com/crowdsecurity/crowdsec/pkg/csconfig"
	"github.com/crowdsecurity/crowdsec/pkg/cwhub"
	"github.com/crowdsecurity/crowdsec/pkg/cwversion"
	"github.com/crowdsecurity/crowdsec/pkg/models"
	"github.com/go-openapi/strfmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"gopkg.in/yaml.v2"
)

var CAPIURLPrefix string = "v2"
var CAPIBaseURL string = "https://api.crowdsec.net/"

func NewCapiCmd() *cobra.Command {
	var cmdCapi = &cobra.Command{
		Use:   "capi [action]",
		Short: "Manage interraction with Central API (CAPI)",
		Args:  cobra.MinimumNArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if csConfig.API.Server == nil {
				log.Fatalln("There is no API->server configuration")
			}
			if csConfig.API.Server.OnlineClient == nil {
				log.Fatalf("no configuration for crowdsec API in '%s'", *csConfig.Self)
			}

			return nil
		},
	}

	var cmdCapiRegister = &cobra.Command{
		Use:   "register",
		Short: "Register to Central API (CAPI)",
		Args:  cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			var err error

			id, err := generateID()
			if err != nil {
				log.Fatalf("unable to generate machine id: %s", err)
			}
			password := strfmt.Password(generatePassword(passwordLength))
			apiurl, err := url.Parse(CAPIBaseURL)
			if err != nil {
				log.Fatalf("unable to parse api url %s : %s", CAPIBaseURL, err)
			}
			_, err = apiclient.RegisterClient(&apiclient.Config{
				MachineID:     id,
				Password:      password,
				UserAgent:     fmt.Sprintf("crowdsec/%s", cwversion.VersionStr()),
				URL:           apiurl,
				VersionPrefix: CAPIURLPrefix,
			}, nil)

			if err != nil {
				log.Fatalf("api client register ('%s'): %s", CAPIBaseURL, err)
			}
			log.Printf("Successfully registered to Central API (CAPI)")

			var dumpFile string

			if outputFile != "" {
				dumpFile = outputFile
			} else if csConfig.API.Server.OnlineClient.CredentialsFilePath != "" {
				dumpFile = csConfig.API.Server.OnlineClient.CredentialsFilePath
			} else {
				dumpFile = ""
			}
			apiCfg := csconfig.ApiCredentialsCfg{
				Login:    id,
				Password: password.String(),
				URL:      CAPIBaseURL,
			}
			apiConfigDump, err := yaml.Marshal(apiCfg)
			if err != nil {
				log.Fatalf("unable to marshal api credentials: %s", err)
			}
			if dumpFile != "" {
				err = ioutil.WriteFile(dumpFile, apiConfigDump, 0600)
				if err != nil {
					log.Fatalf("write api credentials in '%s' failed: %s", dumpFile, err)
				}
				log.Printf("API credentials dumped to '%s'", dumpFile)
			} else {
				fmt.Printf("%s\n", string(apiConfigDump))
			}

			log.Warningf("Run 'systemctl reload crowdsec' for the new configuration to be effective")
		},
	}
	cmdCapiRegister.Flags().StringVarP(&outputFile, "file", "f", "", "output file destination")
	cmdCapi.AddCommand(cmdCapiRegister)

	var cmdCapiStatus = &cobra.Command{
		Use:   "status",
		Short: "Check status with the Central API (CAPI)",
		Args:  cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			if csConfig.API.Server == nil {
				log.Fatalln("There is no configuration on 'api_client:'")
			}
			if csConfig.API.Server.OnlineClient == nil {
				log.Fatalf("Please provide credentials for the API in '%s'", csConfig.API.Server.OnlineClient.CredentialsFilePath)
			}

			if csConfig.API.Server.OnlineClient.Credentials == nil {
				log.Fatalf("no credentials for crowdsec API in '%s'", csConfig.API.Server.OnlineClient.CredentialsFilePath)
			}

			password := strfmt.Password(csConfig.API.Server.OnlineClient.Credentials.Password)
			apiurl, err := url.Parse(csConfig.API.Server.OnlineClient.Credentials.URL)
			if err != nil {
				log.Fatalf("parsing api url ('%s'): %s", csConfig.API.Server.OnlineClient.Credentials.URL, err)
			}
			if err := cwhub.GetHubIdx(csConfig.Cscli); err != nil {
				log.Fatalf("Failed to load hub index : %s", err)
			}
			scenarios, err := cwhub.GetUpstreamInstalledScenariosAsString()
			if err != nil {
				log.Fatalf("failed to get scenarios : %s", err.Error())
			}

			Client, err = apiclient.NewDefaultClient(apiurl, CAPIURLPrefix, fmt.Sprintf("crowdsec/%s", cwversion.VersionStr()), nil)
			if err != nil {
				log.Fatalf("init default client: %s", err)
			}
			t := models.WatcherAuthRequest{
				MachineID: &csConfig.API.Server.OnlineClient.Credentials.Login,
				Password:  &password,
				Scenarios: scenarios,
			}
			log.Infof("Loaded credentials from %s", csConfig.API.Server.OnlineClient.CredentialsFilePath)
			log.Infof("Trying to authenticate with username %s on %s", csConfig.API.Server.OnlineClient.Credentials.Login, apiurl)
			resp, err := Client.Auth.AuthenticateWatcher(context.Background(), t)
			if err != nil {
				log.Fatalf("Failed to authenticate to Central API (CAPI) : %s", err)
			} else {
				log.Infof("You can successfully interact with Central API (CAPI)")
			}
			for k, v := range resp.Response.Header {
				log.Debugf("[headers] %s : %s", k, v)
			}
			dump, _ := httputil.DumpResponse(resp.Response, true)
			log.Debugf("Response: %s", string(dump))
		},
	}
	cmdCapi.AddCommand(cmdCapiStatus)
	return cmdCapi
}
