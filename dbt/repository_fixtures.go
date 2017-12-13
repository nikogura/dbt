package dbt

import "fmt"

func testToolUrl(port int) string {
	return fmt.Sprintf("http://localhost:%d/dbt", port)
}

func dbtIndexOutput() string {
	return `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 3.2 Final//EN">
<html>
<head><title>Index of dbt/dbt</title>
</head>
<body>
<h1>Index of dbt/dbt</h1>
<pre>Name                Last modified      Size</pre><hr/>
<pre><a href="../">../</a>
<a href="1.2.2/">1.2.2/</a>               07-Dec-2017 00:47    -
<a href="1.2.3/">1.2.3/</a>               07-Dec-2017 00:47    -
<a href="install_dbt.sh">install_dbt.sh</a>       07-Dec-2017 00:47  767 bytes
<a href="install_dbt.sh.asc">install_dbt.sh.asc</a>   07-Dec-2017 00:47  516 bytes
</pre>
</body>
</html>`

}
