package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"helm-push-artifactory-plugin/pkg/artifactory"
	"helm-push-artifactory-plugin/pkg/helm"
	helmrepo "helm-push-artifactory-plugin/pkg/repo"
	"helm-push-artifactory-plugin/pkg/version"
)

type (
	pushCmd struct {
		chartName          string
		chartVersion       string
		appVersion         string
		repository         string
		path               string
		username           string
		password           string
		accessToken        string
		apiKey             string
		caFile             string
		certFile           string
		keyFile            string
		insecureSkipVerify bool
		skipReindex        bool
		overrides          []string
	}
)

var (
	globalUsage = `Helm plugin to push chart package to Artifactory
%version%

Examples:

  $ helm push-artifactory mychart-0.1.0.tgz https://artifactory/repo       # push mychart-0.1.0.tgz from "helm package"
  $ helm push-artifactory . https://artifactory/repo                       # package and push chart directory
  $ helm push-artifactory . --version="7c4d121" https://artifactory/repo   # override version in Chart.yaml
  $ helm push-artifactory mychart-0.1.0.tgz my-helm-repo                   # push mychart-0.1.0.tgz to a "my-helm-repo" repository
`
)

func getUsage() string {
	return strings.Replace(globalUsage, "%version%", version.GetVersion(), 1)
}

func newPushCmd(args []string) (*cobra.Command, error) {
	p := &pushCmd{}
	cmd := &cobra.Command{
		Use:          "helm push-artifactory",
		Short:        "Helm plugin to push chart package to Artifactory",
		Long:         getUsage(),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 2 {
				return errors.New("This command needs 2 arguments: name of chart, repository URL")
			}
			p.chartName = args[0]
			p.repository = args[1]
			p.setFieldsFromEnv()
			return p.push()
		},
	}
	f := cmd.Flags()
	f.StringVarP(&p.chartVersion, "version", "v", "", "Override chart version pre-push")
	f.StringVarP(&p.appVersion, "app-version", "", "", "Override chart app version pre-push")
	f.StringArrayVarP(&p.overrides, "set", "s", []string{}, "<key>=<value> pairs, overrides values in chart values.yaml (example: -s image.tag=\"0.5.2\")")
	f.StringVarP(&p.path, "path", "", "", "Path to save the chart in the local repository (https://artifactory/repo/path/chart.version.tgz) [$HELM_REPO_PATH]")
	f.StringVarP(&p.username, "username", "u", "", "Override HTTP basic auth username [$HELM_REPO_USERNAME]")
	f.StringVarP(&p.password, "password", "p", "", "Override HTTP basic auth password [$HELM_REPO_PASSWORD]")
	f.StringVarP(&p.accessToken, "access-token", "", "", "Send token in Authorization header [$HELM_REPO_ACCESS_TOKEN]")
	f.StringVarP(&p.apiKey, "api-key", "", "", "Send api key in artifactory header [$HELM_REPO_API_KEY]")
	f.StringVarP(&p.caFile, "ca-file", "", "", "Verify certificates of HTTPS-enabled servers using this CA bundle [$HELM_REPO_CA_FILE]")
	f.StringVarP(&p.certFile, "cert-file", "", "", "Identify HTTPS client using this SSL certificate file [$HELM_REPO_CERT_FILE]")
	f.StringVarP(&p.keyFile, "key-file", "", "", "Identify HTTPS client using this SSL key file [$HELM_REPO_KEY_FILE]")
	f.BoolVarP(&p.insecureSkipVerify, "insecure", "", false, "Connect to server with an insecure way by skipping certificate verification [$HELM_REPO_INSECURE]")
	f.BoolVarP(&p.skipReindex, "skip-reindex", "", false, "Avoid trigger reindex in the repository after pushing the chart [$HELM_REPO_SKIP_REINDEX]")
	f.Parse(args)
	return cmd, nil
}

func (p *pushCmd) setFieldsFromEnv() {
	if v, ok := os.LookupEnv("HELM_REPO_PATH"); ok && p.path == "" {
		p.path = v
	}
	if v, ok := os.LookupEnv("HELM_REPO_USERNAME"); ok && p.username == "" {
		p.username = v
	}
	if v, ok := os.LookupEnv("HELM_REPO_PASSWORD"); ok && p.password == "" {
		p.password = v
	}
	if v, ok := os.LookupEnv("HELM_REPO_ACCESS_TOKEN"); ok && p.accessToken == "" {
		p.accessToken = v
	}
	if v, ok := os.LookupEnv("HELM_REPO_API_KEY"); ok && p.apiKey == "" {
		p.apiKey = v
	}
	if v, ok := os.LookupEnv("HELM_REPO_CA_FILE"); ok && p.caFile == "" {
		p.caFile = v
	}
	if v, ok := os.LookupEnv("HELM_REPO_CERT_FILE"); ok && p.certFile == "" {
		p.certFile = v
	}
	if v, ok := os.LookupEnv("HELM_REPO_KEY_FILE"); ok && p.keyFile == "" {
		p.keyFile = v
	}
	if v, ok := os.LookupEnv("HELM_REPO_INSECURE"); ok {
		p.insecureSkipVerify, _ = strconv.ParseBool(v)
	}
	if v, ok := os.LookupEnv("HELM_REPO_SKIP_REINDEX"); ok {
		p.skipReindex, _ = strconv.ParseBool(v)
	}
}

func (p *pushCmd) push() error {
	var repo *helmrepo.Repo
	var err error

	// If the argument looks like a URL, just create a temp repo object
	// instead of looking for the entry in the local repository list
	if regexp.MustCompile(`^https?://`).MatchString(p.repository) {
		// Check valid URL
		_, err = url.ParseRequestURI(p.repository)
	} else {
		repo, err = helmrepo.GetRepoByName(p.repository)
	}

	if err != nil {
		return err
	}

	chart, err := helm.GetChartByName(p.chartName)
	if err != nil {
		return err
	}

	// version override
	if p.chartVersion != "" {
		chart.SetVersion(p.chartVersion)
	}

	// app version override
	if p.appVersion != "" {
		chart.SetAppVersion(p.appVersion)
	}

	if len(p.overrides) > 0 {
		err := chart.OverrideValues(p.overrides)
		if err != nil {
			return err
		}
	}

	if repo != nil {
		p.repository = repo.URL
		if p.username == "" {
			p.username = repo.Username
		}
		if p.password == "" {
			p.password = repo.Password
		}
		if p.caFile == "" {
			p.caFile = repo.CAFile
		}
		if p.certFile == "" {
			p.certFile = repo.CertFile
		}
		if p.keyFile == "" {
			p.keyFile = repo.KeyFile
		}
	}

	client, err := artifactory.NewClient(
		artifactory.URL(p.repository),
		artifactory.Path(p.path),
		artifactory.Username(p.username),
		artifactory.Password(p.password),
		artifactory.AccessToken(p.accessToken),
		artifactory.ApiKey(p.apiKey),
		artifactory.CAFile(p.caFile),
		artifactory.CertFile(p.certFile),
		artifactory.KeyFile(p.keyFile),
		artifactory.InsecureSkipVerify(p.insecureSkipVerify),
	)

	if err != nil {
		return err
	}

	tmp, err := ioutil.TempDir("", "helm-push-artifactory-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	chartPackagePath, err := helm.CreateChartPackage(chart, tmp)
	if err != nil {
		return err
	}

	resp, err := client.UploadChartPackage(chart.Metadata.Name, chartPackagePath)
	if err != nil {
		return err
	}

	if err = handlePushResponse(resp); err != nil {
		return err
	}

	if p.skipReindex {
		return nil
	}

	resp, err = client.ReindexArtifactoryRepo()
	if err != nil {
		return err
	}
	return handleReindexResponse(resp)
}

func handleReindexResponse(resp *http.Response) error {
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return getArtifactoryError(b, resp.StatusCode)
	}
	fmt.Println(string(b))
	return nil
}

func handlePushResponse(resp *http.Response) error {
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return getArtifactoryError(b, resp.StatusCode)
	}
	fmt.Println("Done.")
	return nil
}

type artifactoryError struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

type errorResponse struct {
	Errors []artifactoryError `json:"errors"`
}

func getArtifactoryError(b []byte, code int) error {
	response := errorResponse{}
	err := json.Unmarshal(b, &response)
	if err != nil || len(response.Errors) == 0 {
		return fmt.Errorf("%d: could not properly parse response JSON: %s", code, string(b))
	}

	return fmt.Errorf("%d: %s", code, response.Errors[0].Message)
}

func main() {
	cmd, err := newPushCmd(os.Args[1:])
	if err != nil {
		fmt.Println(fmt.Sprintf("%v", err))
	}
	if err := cmd.Execute(); err != nil {
		fmt.Println(fmt.Sprintf("%v", err))
		os.Exit(1)
	}
}
