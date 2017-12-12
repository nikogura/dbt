package dbt

func testDbtConfigContents() string {
	return `{
  "dbt": {
    "repository": "http://localhost:8081/artifactory/dbt",
    "truststore": "http://localhost:8081/artifactory/dbt/truststore"
  },
  "tools": {
    "repository": "http://localhost:8081/artifactory/dbt-tools"
  }
}`
}

func testDbtConfig() Config {
	return Config{
		DbtConfig{
			Repo:       "http://localhost:8081/artifactory/dbt",
			TrustStore: "http://localhost:8081/artifactory/dbt/truststore",
		},
		ToolsConfig{
			Repo: "http://localhost:8081/artifactory/dbt-tools",
		},
	}
}
