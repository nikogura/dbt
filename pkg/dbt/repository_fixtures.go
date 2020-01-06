// Copyright © 2019 Nik Ogura <nik.ogura@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dbt

import "fmt"

func testDbtUrl(port int) string {
	return fmt.Sprintf("http://localhost:%d/dbt", port)
}

func testToolUrl(port int) string {
	return fmt.Sprintf("http://localhost:%d/dbt-tools", port)
}

func dbtIndexOutput() string {
	return `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 3.2 Final//EN">
<html>
<head><title>Index of dbt/foo</title>
</head>
<body>
<h1>Index of dbt/foo</h1>
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

//func dbtVersionAIndexOutput() string {
//	return `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 3.2 Final//EN">
//<html>
//<head><title>Index of dbt-tools/foo/1.2.2/</title>
//</head>
//<body>
//<h1>Index of dbt/dbt</h1>
//<pre>Name                Last modified      Size</pre><hr/>
//<pre><a href="../">../</a>
//<a href="darwin/">darwin/</a>               07-Dec-2017 00:47    -
//<a href="linux/">linux/</a>               07-Dec-2017 00:47    -
//</pre>
//</body>
//</html>`
//
//}

func dbtVersionAContent() string {
	return "The quick fox jumped over the lazy brown dog."
}

func dbtVersionASha256() string {
	return "1b47f99f277cad8c5e31f21e688e4d0b8803cb591b0383e2319869b520d061a1"
}

//func dbtVersionASha1() string {
//	return "5b7c9753dd9800a16969bf65e2330b40e657277b"
//}

//func dbtVersionAMd5() string {
//	return "6e8bafd13ba44c93b63cead889cd21a6"
//}

func dbtVersionASig() string {
	return `-----BEGIN PGP SIGNATURE-----

iQFABAABCAAqFiEE3Ww86tgfSQ9lgLSizmhGNf2l0x8FAlpAH58MHGRidEBkYnQu
Y29tAAoJEM5oRjX9pdMf9ZwH/1xC8SKBhW7qs49z48GMb35U6l6n7FakZZGmn6Z6
fxr4haUPGPF+HSN6gaGfxUqHQnFrfRHHt4fsyjpReyb2I8A2GP/0MHqFiEEMLXIk
tZa3Qp+BJo93ofjZM4fVKNgF6qDHzkD8vg0djzTgiNbOntR0fHXaDAOxbwYGgogd
JtOaAYTXZ1ToP2X9mog8PlOYeXAYctD+5KBUOmRAxy4nK8Fn+NFXY7U6tF9+baV4
h9gCZS4HiPoLx4HmK0zMYf+sn+izHf0NdgsR1d2h5ojqlykNsQ8ejWENiEZVnoHL
D6RsxGI6qFwXx7usp5L2IzW5q1BI12kGA782eY3Op0+DtVI=
=6pax
-----END PGP SIGNATURE-----
`
}

func dbtVersionBIndexOutput() string {
	return `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 3.2 Final//EN">
<html>
<head><title>Index of dbt-tools/foo/1.2.3/</title>
</head>
<body>
<h1>Index of dbt/dbt</h1>
<pre>Name                Last modified      Size</pre><hr/>
<pre><a href="../">../</a>
<a href="darwin/">darwin/</a>               07-Dec-2017 00:47    -
<a href="linux/">linux/</a>               07-Dec-2017 00:47    -
</pre>
</body>
</html>`

}

func dbtVersionBContent() string {
	return "Twas brillig, and the slithy toves did gyre and gimble in the wabe."
}

func dbtVersionBSha256() string {
	return "f1f4f38ccbc574d457110f8994d5a127d6441aa915ddfc3247a1132e212fcda0"
}

//func dbtVersionBSha1() string {
//	return "2b0c08dd5c80e654b2ce0c4b86fd9e29e8cd479a"
//}

//func dbtVersionBMd5() string {
//	return "38c3f92660f1cd49bcb428ee2932000e"
//}
//
//func dbtVersionBSig() string {
//	return `-----BEGIN PGP SIGNATURE-----
//
//iQFABAABCAAqFiEE3Ww86tgfSQ9lgLSizmhGNf2l0x8FAlpAH1cMHGRidEBkYnQu
//Y29tAAoJEM5oRjX9pdMfDl4IALE2Oj1r25rzL22NGr7Ip3Fmo5joZXUpeO6l2/05
//1PBbEIM6qGdvd2lXvkaROFhRNwreqYI8f1bt7l3MCATvE9lycWcosjDdnGIijJzV
//qFy4HL0aGnYsrLhQWn+3F6AYbObp1SF4SchwC96B2+z4YD6Pty/8U8ielkTWsFUz
//75xJem+CYKjJxk68lVIDzxLO5w7K4P/7Z9l0bevzzZcky4TR13w7EpJ2EkqYR4MS
///Sevt+l1nded5CUFjZxhlAosvpCusXG7hk6TFV1sQKh4XJNelhU818QXV6BGG59B
//A7vLjTabSUqvwhWHa0S3bOn196RyyCOb28y7+2EOG3jIJGU=
//=RXqm
//-----END PGP SIGNATURE-----
//`
//}
