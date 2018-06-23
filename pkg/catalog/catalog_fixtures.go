package catalog

import (
	"fmt"
	"github.com/nikogura/dbt/pkg/dbt"
)

func testDbtConfig(port int) dbt.Config {
	config := dbt.Config{}

	config.Dbt = dbt.DbtConfig{
		Repo:       fmt.Sprintf("http://localhost:%d/dbt", port),
		TrustStore: fmt.Sprintf("http://localhost:%d/dbt/truststore", port),
	}

	config.Tools = dbt.ToolsConfig{
		Repo: fmt.Sprintf("http://localhost:%d/dbt-tools", port),
	}

	return config
}

func repoIndex() string {
	return `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 3.2 Final//EN">
<html>
<head><title>Index of dbt-tools</title>
</head>
<body>
<h1>Index of dbt</h1>
<pre>Name                Last modified      Size</pre><hr/>
<pre><a href="../">../</a>
<a href="foo/">foo/</a>               07-Dec-2017 00:47    -
<a href="bar/">bar/</a>               07-Dec-2017 00:47    -
</pre>
</body>
</html>`

}

func fooIndex() string {
	return `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 3.2 Final//EN">
<html>
<head><title>Index of dbt-tools/foo</title>
</head>
<body>
<h1>Index of dbt/foo</h1>
<pre>Name                Last modified      Size</pre><hr/>
<pre><a href="../">../</a>
<a href="1.2.2/">1.2.2/</a>               07-Dec-2017 00:47    -
<a href="1.2.3/">1.2.3/</a>               07-Dec-2017 00:47    -
</pre>
</body>
</html>`

}

func barIndex() string {
	return `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 3.2 Final//EN">
<html>
<head><title>Index of dbt-tools/bar</title>
</head>
<body>
<h1>Index of dbt/bar</h1>
<pre>Name                Last modified      Size</pre><hr/>
<pre><a href="../">../</a>
<a href="1.1.1/">1.1.1/</a>               07-Dec-2017 00:47    -
</pre>
</body>
</html>`

}

func fooDescription() string {
	return "frobnitz ene woo"
}

func barDescription() string {
	return "goongala goongala"
}

func fooTool() Tool {
	return Tool{Name: "foo"}
}

func barTool() Tool {
	return Tool{Name: "bar"}
}

func testTools() []Tool {
	list := make([]Tool, 0)
	list = append(list, fooTool())
	list = append(list, barTool())

	return list
}

func testDbtConfigContents(port int) string {
	return fmt.Sprintf(`{
  "dbt": {
    "repository": "http://localhost:%d/dbt",
    "truststore": "http://localhost:%d/dbt/truststore"
  },
  "tools": {
    "repository": "http://localhost:%d/dbt-tools"
  }
}`, port, port, port)
}
