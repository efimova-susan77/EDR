package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/0xrawsec/gene/v2/engine"
	"github.com/0xrawsec/golang-utils/code/builder"
	"github.com/0xrawsec/whids/ioc"
	"github.com/0xrawsec/whids/openapi"
)

const (
	guid      = "5a92baeb-9384-47d3-92b4-a0db6f9b8c6d"
	eventHash = "3d8441643c204ba9b9dcb5c414b25a3129f66f6c"
)

var (
	openAPI = openapi.New(
		"3.0.2",
		&openapi.Info{
			Title:   "WHIDS API documentation",
			Version: "1.0",
		},
		&openapi.Server{
			URL: mconf.AdminAPIUrl(),
		})
)

func init() {
	openAPI.AuthApiKey(AuthKeyHeader, testAdminUser.Key)
	openAPI.Client = &http.Client{Transport: cconf.Transport()}
	Hostname = "OpenHappy"
}

func prep() (m *Manager, c *ManagerClient) {
	var err error

	key := KeyGen(DefaultKeySize)

	if m, err = NewManager(&mconf); err != nil {
		panic(err)
	}

	// we don't handle error as we don't care if user
	// already exists
	m.CreateNewAdminAPIUser(testAdminUser)

	m.AddEndpoint(cconf.UUID, key)
	m.Run()

	cconf.Key = key
	if c, err = NewManagerClient(&cconf); err != nil {
		panic(err)
	}
	// wait that server is up
	// might generate error message in log
	for !c.IsServerUp() {
		time.Sleep(time.Nanosecond * 500)
	}
	fconf.Client.Key = key

	// sending logs to manager
	f, err := NewForwarder(&fconf)
	if err != nil {
		panic(err)
	}
	f.Run()
	defer f.Close()

	for e := range emitMixedEvents(50, 50) {
		f.PipeEvent(e)
	}

	return
}

func runAdminApiTest(t *testing.T, f func(*testing.T)) {
	m, c := prep()

	// Create fake dumps
	for _, name := range []string{"foo.txt", "bar.txt"} {
		c.PostDump(&FileUpload{
			Name:      name,
			GUID:      fmt.Sprintf("{%s}", guid),
			EventHash: eventHash,
			Content:   []byte("Blah"),
			Chunk:     1,
			Total:     1,
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for ctx.Err() == nil {
			if c.IsServerUp() {
				if cmd, err := c.FetchCommand(); err != nil && err != ErrNothingToDo {
					t.Logf("Client failed to fetch command: %s", err)
					break
				} else {
					if err := cmd.Run(); err != nil {
						t.Logf("Failed to run command: %s", err)
						break
					}
					if err := c.PostCommand(cmd); err != nil {
						t.Logf("Failed to post command: %s", err)
						break
					}
				}
			}
			time.Sleep(time.Second)
		}
	}()

	defer func() {
		cancel()
		m.Shutdown()
		m.Wait()
		os.RemoveAll(m.Config.Database)
		os.RemoveAll(m.Config.DumpDir)
	}()
	f(t)
}

func writeConfig(filename string, data []byte) {
	f, err := os.Create(filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	f.Write(data)
}

func TestOpenApi(t *testing.T) {
	f := func(t *testing.T) {
		// User Management
		usersPath := openapi.PathItem{
			Summary: "Admin API User's Management",
			Value:   AdmAPIUsers,
		}

		openAPI.Do(usersPath,
			openapi.Operation{
				Method:  "GET",
				Summary: "List all users",
				Output:  AdminAPIResponse{},
			},
		)

		openAPI.Do(usersPath,
			openapi.Operation{
				Method:  "PUT",
				Summary: "Create a new user with identifier",
				Parameters: []*openapi.Parameter{
					openapi.QueryParameter(qpIdentifier, "TestAdminUser").Require(),
				},
				Output: AdminAPIResponse{},
			},
		)

		guid := UUIDGen().String()
		openAPI.Do(usersPath,
			openapi.Operation{
				Method:  "POST",
				Summary: "Create a new user from POST data",
				RequestBody: openapi.JsonRequestBody(
					"Data to create the user with. Fields uuid and key if empty will be generated.",
					AdminAPIUser{
						Identifier:  "SecondTestAdmin",
						Uuid:        guid,
						Key:         "ChangeMe",
						Description: "Second admin user",
						Group:       "CSIRT",
					}, true),
				Output: AdminAPIResponse{},
			},
		)

		openAPI.Do(usersPath,
			openapi.Operation{
				Method:  "POST",
				Summary: "Modify existing admin API user",
				Parameters: []*openapi.Parameter{
					openapi.PathParameter("uuid", guid),
					openapi.QueryParameter(qpNewKey, true, "Generate a new random key for user").Skip(),
				},
				RequestBody: openapi.JsonRequestBody(
					"Data to update user with",
					AdminAPIUser{
						Key:         "NewWeakKey",
						Description: "Second admin user changed",
						Group:       "SOC",
					}, true),
			})

		openAPI.Do(usersPath,
			openapi.Operation{
				Method:  "DELETE",
				Summary: "Delete an existing admin API user",
				Parameters: []*openapi.Parameter{
					openapi.PathParameter("uuid", guid),
				},
			})
	}

	runAdminApiTest(t, f)
}

func TestOpenApiEndpointManagement(t *testing.T) {
	f := func(t *testing.T) {

		// Endpoint Management
		endpointPath := openapi.PathItem{
			Summary: "Endpoint Management",
			Value:   AdmAPIEndpointsPath,
		}

		openAPI.Do(endpointPath, openapi.Operation{
			Method:  "GET",
			Summary: "Get endpoints",
			Parameters: []*openapi.Parameter{
				openapi.QueryParameter(qpShowKey, true, "Show or not key"),
				openapi.QueryParameter(qpGroup, "", "Filter by group"),
				openapi.QueryParameter(qpStatus, "", "Filter by status"),
				openapi.QueryParameter(qpCriticality, 0, "Filter by criticality"),
			},
			Output: AdminAPIResponse{},
		})

		openAPI.Do(endpointPath, openapi.Operation{
			Method:  "PUT",
			Summary: "Create a new endpoint",
			Output:  AdminAPIResponse{},
		})

		openAPI.Do(endpointPath, openapi.Operation{
			Method:  "GET",
			Summary: "Get information about a single endpoint",
			Parameters: []*openapi.Parameter{
				openapi.PathParameter("uuid", cconf.UUID),
			},
			Output: AdminAPIResponse{},
		})

		openAPI.Do(endpointPath, openapi.Operation{
			Method:  "POST",
			Summary: "Modify an existing endpoint",
			Parameters: []*openapi.Parameter{
				openapi.PathParameter("uuid", cconf.UUID),
				openapi.QueryParameter(qpShowKey, true, "Show endpoint key in response").Skip(),
				openapi.QueryParameter(qpNewKey, true, "Generate a new key for endpoint").Skip(),
			},
			RequestBody: openapi.JsonRequestBody(
				"Fields to modify. NB: Not all the fields can be modified",
				Endpoint{
					Key:    "New Key",
					Group:  "New Group",
					Status: "New Status",
				}, true),
			Output: AdminAPIResponse{},
		})

		// Delete endpoint after everything
		openAPI.Do(endpointPath, openapi.Operation{
			Method:  "DELETE",
			Summary: "Delete an existing endpoint",
			Parameters: []*openapi.Parameter{
				openapi.PathParameter("uuid", cconf.UUID),
			},
			Output: AdminAPIResponse{},
		})
	}

	runAdminApiTest(t, f)
}

func TestOpenApiEndpointCommands(t *testing.T) {
	f := func(t *testing.T) {

		endpointPath := openapi.PathItem{
			Summary: "Endpoint Execution",
			Value:   AdmAPIEndpointsPath,
		}

		openAPI.Do(endpointPath, openapi.Operation{
			Method:  "POST",
			Summary: "Send a command to be executed by the endpoint",
			Parameters: []*openapi.Parameter{
				openapi.PathParameter("uuid", cconf.UUID).Suffix("/command"),
			},
			RequestBody: openapi.JsonRequestBody(
				`Command to be executed. One can also specify files 
				to drop from the manager to the endpoint prior to command execution 
				and files to fetch after execution. A timeout for the can also 
				be specified, if zero there will be no timeout.`,
				CommandAPI{CommandLine: `printf "Hello World"`},
				true),
			Output: AdminAPIResponse{},
		})

		openAPI.Do(endpointPath, openapi.Operation{
			Method:  "GET",
			Summary: "Get the result of a command executed on endpoint",
			Parameters: []*openapi.Parameter{
				openapi.QueryParameter(
					qpWait,
					true,
					"Wait command to end before responding, making the call blocking"),
				openapi.PathParameter("uuid", cconf.UUID).Suffix(AdmAPICommandSuffix),
			},
			Output: AdminAPIResponse{},
		})

		openAPI.Do(endpointPath, openapi.Operation{
			Method:  "GET",
			Summary: "Retrieve only a field of the command structure",
			Parameters: []*openapi.Parameter{
				openapi.QueryParameter(
					qpWait,
					true,
					"Wait command to end before responding, making the call blocking").Skip(),
				openapi.PathParameter("uuid",
					cconf.UUID).Suffix(AdmAPICommandSuffix),
				openapi.PathParameter("field",
					"stdout",
					"Field of the Command structure to return"),
			},
			Output: AdminAPIResponse{},
		})
	}

	runAdminApiTest(t, f)
}

func TestOpenApiReports(t *testing.T) {
	f := func(t *testing.T) {

		sum := "Detection Reports"
		endpointsPath := openapi.PathItem{
			Summary: sum,
			Value:   AdmAPIEndpointsPath,
		}

		reportsPath := openapi.PathItem{
			Summary: sum,
			Value:   AdmAPIEndpointsReportsPath,
		}

		openAPI.Do(reportsPath, openapi.Operation{
			Method:  "GET",
			Summary: "Get all detection reports",
			Output:  AdminAPIResponse{},
		})

		openAPI.Do(endpointsPath, openapi.Operation{
			Method:  "GET",
			Summary: "Retrieve report for a single endpoint",
			Parameters: []*openapi.Parameter{
				openapi.PathParameter("uuid",
					cconf.UUID).Suffix(AdmAPIReportSuffix)},
			Output: AdminAPIResponse{},
		})

		openAPI.Do(endpointsPath, openapi.Operation{
			Method:  "DELETE",
			Summary: "Delete and archive a report for a single endpoint",
			Parameters: []*openapi.Parameter{
				openapi.PathParameter("uuid",
					cconf.UUID).Suffix(AdmAPIReportSuffix)},
			Output: AdminAPIResponse{},
		})

		openAPI.Do(endpointsPath, openapi.Operation{
			Method:  "GET",
			Summary: "Get archived reports",
			Parameters: []*openapi.Parameter{
				openapi.QueryParameter(qpSince, time.Now().Format(time.RFC3339), "Retrieve report since date (RFC3339)"),
				openapi.QueryParameter(qpUntil, time.Now().Format(time.RFC3339), "Retrieve report until date (RFC3339)"),
				openapi.QueryParameter(qpLast, "1d", "Return last reports from duration (ex: `1d` for last day)"),
				openapi.QueryParameter(qpLimit, 42, "Maximum number of reports to return"),
				openapi.PathParameter("uuid",
					cconf.UUID).Suffix(AdmAPIReportSuffix).Suffix(AdmAPIArchiveSuffix)},
			Output: AdminAPIResponse{},
		})

	}

	runAdminApiTest(t, f)
}

func TestOpenApiLogs(t *testing.T) {
	f := func(t *testing.T) {

		logsPath := openapi.PathItem{
			Summary: "Endpoint Log Retrieval",
			Value:   AdmAPIEndpointsPath,
		}

		nowStr := time.Now().Format(time.RFC3339)

		openAPI.Do(logsPath, openapi.Operation{
			Method:  "GET",
			Summary: "Retrieve any kind of logs (detections + filtered)",
			Parameters: []*openapi.Parameter{
				openapi.QueryParameter(qpSince, nowStr, "Retrieve logs since date (RFC3339)").Skip(),
				openapi.QueryParameter(qpUntil, nowStr, "Retrieve logs until date (RFC3339)").Skip(),
				openapi.QueryParameter(qpLast, "1d", "Return last logs from duration (ex: `1d` for last day)"),
				openapi.QueryParameter(qpPivot, nowStr, "Timestamp to pivot around (RFC3339)"),
				openapi.QueryParameter(qpDelta, "5m", "Delta duration used to pivot (ex: `5m` to get logs 5min around pivot) "),
				openapi.QueryParameter(qpLimit, 2, "Maximum number of reports to return"),
				openapi.QueryParameter(qpSkip, 0, "Skip number of events").Skip(),
				openapi.PathParameter("uuid",
					cconf.UUID).Suffix(AdmAPILogsSuffix)},
			Output: AdminAPIResponse{},
		})

		openAPI.Do(logsPath, openapi.Operation{
			Method:  "GET",
			Summary: "Retrieve detections logs",
			Parameters: []*openapi.Parameter{
				openapi.QueryParameter(qpSince, nowStr, "Retrieve logs since date (RFC3339)").Skip(),
				openapi.QueryParameter(qpUntil, nowStr, "Retrieve logs until date (RFC3339)").Skip(),
				openapi.QueryParameter(qpLast, "1d", "Return last logs from duration (ex: `1d` for last day)"),
				openapi.QueryParameter(qpPivot, nowStr, "Timestamp to pivot around (RFC3339)"),
				openapi.QueryParameter(qpDelta, "5m", "Delta duration used to pivot (ex: `5m` to get logs 5min around pivot) "),
				openapi.QueryParameter(qpLimit, 2, "Maximum number of reports to return"),
				openapi.QueryParameter(qpSkip, 0, "Skip number of events").Skip(),
				openapi.PathParameter("uuid",
					cconf.UUID).Suffix(AdmAPIDetectionSuffix),
			},
			Output: AdminAPIResponse{},
		})

	}

	runAdminApiTest(t, f)
}

func TestOpenApiArtifacts(t *testing.T) {
	f := func(t *testing.T) {

		sum := "Artifact Search and Retrieval"
		artifactsPath := openapi.PathItem{
			Summary: sum,
			Value:   AdmAPIEndpointsArtifactsPath,
		}

		endpointsPath := openapi.PathItem{
			Summary: sum,
			Value:   AdmAPIEndpointsPath,
		}

		nowStr := time.Now().Format(time.RFC3339)

		openAPI.Do(artifactsPath, openapi.Operation{
			Method:  "GET",
			Summary: "Artifacts on all endpoints",
			Parameters: []*openapi.Parameter{
				openapi.QueryParameter(qpSince, nowStr, "Retrieve artifacts received since date (RFC3339)").Skip()},
			Output: AdminAPIResponse{},
		})

		openAPI.Do(endpointsPath, openapi.Operation{
			Method:  "GET",
			Summary: "Artifacts for a single endpoint",
			Parameters: []*openapi.Parameter{
				openapi.QueryParameter(qpSince, nowStr, "Retrieve artifacts received since date (RFC3339)").Skip(),
				openapi.PathParameter("uuid", cconf.UUID).Suffix(AdmAPIArticfactsSuffix),
			},
			Output: AdminAPIResponse{},
		})

		openAPI.Do(endpointsPath, openapi.Operation{
			Method:  "GET",
			Summary: "Retrieve the content of an artifact",
			Parameters: []*openapi.Parameter{
				openapi.QueryParameter(qpRaw, false, "Retrieve raw file content").Skip(),
				openapi.QueryParameter(qpGunzip, false, "Serve gunziped file content").Skip(),
				openapi.PathParameter("uuid", cconf.UUID).Suffix(AdmAPIArticfactsSuffix),
				openapi.PathParameter("pguid", guid),
				openapi.PathParameter("ehash", eventHash),
				openapi.PathParameter("filename", "foo.txt"),
			},
			Output: AdminAPIResponse{},
		})

	}

	runAdminApiTest(t, f)
}

func TestOpenApiIoCs(t *testing.T) {
	f := func(t *testing.T) {

		iocsPath := openapi.PathItem{
			Summary: "IoC Management (control IoCs pushed on Endpoints)",
			Value:   AdmAPIIocsPath,
		}
		provider := "XyzTIProvider"

		openAPI.Do(iocsPath, openapi.Operation{
			Method:  "POST",
			Summary: "Add IoCs to be pushed on endpoints for detection",
			RequestBody: openapi.JsonRequestBody("",
				[]ioc.IoC{
					{
						Source: provider,
						Key:    UUIDGen().String(),
						Value:  "some.random.domain",
						Type:   "domain",
					},
				},
				true),
			Output: AdminAPIResponse{},
		})

		openAPI.Do(iocsPath, openapi.Operation{
			Method:  "GET",
			Summary: "Query IoCs loaded on manager and currently pushed to endpoints",
			Parameters: []*openapi.Parameter{
				openapi.QueryParameter(qpSource, "Test", "Filter by source").Skip(),
				openapi.QueryParameter(qpKey, "Test", "Filter by key").Skip(),
				openapi.QueryParameter(qpValue, "Test", "Filter by value").Skip(),
				openapi.QueryParameter(qpType, "Test", "Filter by type").Skip(),
			},
			Output: AdminAPIResponse{},
		})

		openAPI.Do(iocsPath, openapi.Operation{
			Method: "DELETE",
			Summary: `Delete IoCs from manager, modulo a synchronization delay, endpoints should 
			stop using those for detection`,
			Parameters: []*openapi.Parameter{
				openapi.QueryParameter(qpSource, provider, "Filter by source"),
				openapi.QueryParameter(qpKey, "Test", "Filter by key").Skip(),
				openapi.QueryParameter(qpValue, "Test", "Filter by value").Skip(),
				openapi.QueryParameter(qpType, "Test", "Filter by type").Skip(),
			},
			Output: AdminAPIResponse{},
		})

	}

	runAdminApiTest(t, f)
}

func TestOpenApiRules(t *testing.T) {
	f := func(t *testing.T) {

		sum := "Rules Management"
		rulesPath := openapi.PathItem{
			Summary: sum,
			Value:   AdmAPIRulesPath,
		}

		name := "TestRule"
		openAPI.Do(rulesPath, openapi.Operation{
			Method:  "POST",
			Summary: "Add or modify a rule",
			Parameters: []*openapi.Parameter{
				openapi.QueryParameter(qpUpdate, true, "Update rule if already existing"),
			},
			RequestBody: openapi.JsonRequestBody(
				"Rule to add to the manager",
				engine.Rule{
					Name: name,
					Meta: engine.MetaSection{
						Events:      map[string][]int64{"Microsoft-Windows-Sysmon/Operational": {11, 23, 26}},
						Criticality: 10,
						Schema:      engine.ParseVersion("2.0.0"),
					},
					Matches: []string{
						fmt.Sprintf("$foo: Image ~= '%s'", `C:\\Malware.exe`),
						fmt.Sprintf("$bar: TargetFilename ~= '%s'", `C:\\config.txt`),
					},
					Condition: "$foo or $bar",
					Actions:   []string{"memdump", "kill"},
				},
				true),
			Output: AdminAPIResponse{},
		})

		// ToDo find a way to get a single rule name
		openAPI.Do(rulesPath, openapi.Operation{
			Method:  "GET",
			Summary: "Get rules loaded on endpoints",
			Parameters: []*openapi.Parameter{
				openapi.QueryParameter(qpName, name, "Regex matching the names of the rules to retrieve"),
				openapi.QueryParameter(qpFilters, false, "Show only filters (rules used to filter-in logs)"),
			},
			Output: AdminAPIResponse{},
		})

		openAPI.Do(rulesPath, openapi.Operation{
			Method:  "DELETE",
			Summary: "Delete rules from manager",
			Parameters: []*openapi.Parameter{
				openapi.QueryParameter(qpName, name, "Regex matching the names of the rules to delete"),
			},
			Output: AdminAPIResponse{},
		})

		// Save
		openAPI.Do(
			openapi.PathItem{Summary: sum, Value: AdmAPIRulesSavePath},
			openapi.Operation{
				Method:  "GET",
				Summary: "Save rules for persistence",
				Output:  AdminAPIResponse{},
			})

		// Reload
		openAPI.Do(
			openapi.PathItem{Summary: sum, Value: AdmAPIRulesReloadPath},
			openapi.Operation{
				Method:  "GET",
				Summary: "Reload rules to engine",
				Output:  AdminAPIResponse{},
			})

	}

	runAdminApiTest(t, f)
	t.Log(prettyJSON(openAPI))
}

func TestOpenApiTemplate(t *testing.T) {
	f := func(t *testing.T) {

		path := openapi.PathItem{
			Summary: "Statistics about the manager",
			Value:   AdmAPIStatsPath,
		}

		openAPI.Do(path, openapi.Operation{
			Method:  "GET",
			Summary: "Get statistics",
			Output:  AdminAPIResponse{},
		})

	}

	runAdminApiTest(t, f)
}

/*
func TestOpenApiTemplate(t *testing.T) {
	f := func(t *testing.T) {

		path := openapi.PathItem{
			Summary: "Summary",
			Value:   AdmAPIEndpointsArtifactsPath,
		}

		openAPI.Do(path, openapi.Operation{
			Method:  "GET",
			Summary: "Artifacts on all endpoints",
			Parameters: []*openapi.Parameter{
				openapi.QueryParameter("", "", ""),
			},
			Output: AdminAPIResponse{},
		})

	}

	runAdminApiTest(t, f)
}
*/

func TestOpenApiFinisher(t *testing.T) {
	b := builder.CodeBuilder{}
	b.Package("api")
	b.WriteString("\n")
	b.DefVariable("OpenAPIDefinition", "\n"+prettyJSON(openAPI)+"\n")
	writeConfig("openapi_def.go", b.Bytes())
}