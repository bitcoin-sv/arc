---
title: ARC - Authoritative Response Component v1.0.0
language_tabs:
  - http: HTTP
  - javascript: JavaScript
  - java: Java
  - go: Go
  - ruby: Ruby
  - python: Python
  - shell: curl
language_clients:
  - http: ""
  - javascript: ""
  - java: ""
  - go: ""
  - ruby: ""
  - python: ""
  - shell: ""
toc_footers: []
includes: []
search: false
highlight_theme: darkula
headingLevel: 2

---

<!-- Generator: Widdershins v4.0.1 -->

<h1 id="arc-authoritative-response-component">ARC - Authoritative Response Component v1.0.0</h1>

> Scroll down for code samples, example requests and responses. Select a language for code samples from the tabs above or the mobile navigation menu.

Base URLs:

* <a href="https://arc.taal.com">https://arc.taal.com</a>

License: <a href="https://bitcoinassociation.net/open-bsv-license/">Open BSV Licence</a>

# Authentication

- HTTP Authentication, scheme: bearer Bearer authentication as defined in RFC 6750

<h1 id="arc-authoritative-response-component-arc">Arc</h1>

## Get the policy settings

<a id="opIdGET policy"></a>

> Code samples

```http
GET https://arc.taal.com/v1/policy HTTP/1.1
Host: arc.taal.com
Accept: application/json

```

```javascript

const headers = {
  'Accept':'application/json',
  'Authorization':'Bearer {access-token}'
};

fetch('https://arc.taal.com/v1/policy',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

```java
URL obj = new URL("https://arc.taal.com/v1/policy");
HttpURLConnection con = (HttpURLConnection) obj.openConnection();
con.setRequestMethod("GET");
int responseCode = con.getResponseCode();
BufferedReader in = new BufferedReader(
    new InputStreamReader(con.getInputStream()));
String inputLine;
StringBuffer response = new StringBuffer();
while ((inputLine = in.readLine()) != null) {
    response.append(inputLine);
}
in.close();
System.out.println(response.toString());

```

```go
package main

import (
       "bytes"
       "net/http"
)

func main() {

    headers := map[string][]string{
        "Accept": []string{"application/json"},
        "Authorization": []string{"Bearer {access-token}"},
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("GET", "https://arc.taal.com/v1/policy", data)
    req.Header = headers

    client := &http.Client{}
    resp, err := client.Do(req)
    // ...
}

```

```ruby
require 'rest-client'
require 'json'

headers = {
  'Accept' => 'application/json',
  'Authorization' => 'Bearer {access-token}'
}

result = RestClient.get 'https://arc.taal.com/v1/policy',
  params: {
  }, headers: headers

p JSON.parse(result)

```

```python
import requests
headers = {
  'Accept': 'application/json',
  'Authorization': 'Bearer {access-token}'
}

r = requests.get('https://arc.taal.com/v1/policy', headers = headers)

print(r.json())

```

```shell
# You can also use wget
curl -X GET https://arc.taal.com/v1/policy \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

`GET /v1/policy`

This endpoint returns the policy settings.

> Example responses

> 200 Response

```json
{
  "timestamp": "2019-08-24T14:15:22Z",
  "policy": {
    "maxscriptsizepolicy": 500000,
    "maxtxsigopscountspolicy": 4294967295,
    "maxtxsizepolicy": 10000000,
    "miningFee": {
      "satoshis": 1,
      "bytes": 1000
    }
  }
}
```

<h3 id="get-the-policy-settings-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Success|[PolicyResponse](#schemapolicyresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Security requirements failed|None|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
BearerAuth, None, None
</aside>

## Get metamorph health

<a id="opIdGET health"></a>

> Code samples

```http
GET https://arc.taal.com/v1/health HTTP/1.1
Host: arc.taal.com
Accept: application/json

```

```javascript

const headers = {
  'Accept':'application/json',
  'Authorization':'Bearer {access-token}'
};

fetch('https://arc.taal.com/v1/health',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

```java
URL obj = new URL("https://arc.taal.com/v1/health");
HttpURLConnection con = (HttpURLConnection) obj.openConnection();
con.setRequestMethod("GET");
int responseCode = con.getResponseCode();
BufferedReader in = new BufferedReader(
    new InputStreamReader(con.getInputStream()));
String inputLine;
StringBuffer response = new StringBuffer();
while ((inputLine = in.readLine()) != null) {
    response.append(inputLine);
}
in.close();
System.out.println(response.toString());

```

```go
package main

import (
       "bytes"
       "net/http"
)

func main() {

    headers := map[string][]string{
        "Accept": []string{"application/json"},
        "Authorization": []string{"Bearer {access-token}"},
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("GET", "https://arc.taal.com/v1/health", data)
    req.Header = headers

    client := &http.Client{}
    resp, err := client.Do(req)
    // ...
}

```

```ruby
require 'rest-client'
require 'json'

headers = {
  'Accept' => 'application/json',
  'Authorization' => 'Bearer {access-token}'
}

result = RestClient.get 'https://arc.taal.com/v1/health',
  params: {
  }, headers: headers

p JSON.parse(result)

```

```python
import requests
headers = {
  'Accept': 'application/json',
  'Authorization': 'Bearer {access-token}'
}

r = requests.get('https://arc.taal.com/v1/health', headers = headers)

print(r.json())

```

```shell
# You can also use wget
curl -X GET https://arc.taal.com/v1/health \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

`GET /v1/health`

Checks if metamorph is healthy and running

> Example responses

> 200 Response

```json
{
  "healthy": false,
  "version": "v1.0.0",
  "reason": "no db connection"
}
```

<h3 id="get-metamorph-health-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Success|[Health](#schemahealth)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Security requirements failed|None|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
BearerAuth, None, None
</aside>

## Get transaction status.

<a id="opIdGET transaction status"></a>

> Code samples

```http
GET https://arc.taal.com/v1/tx/{txid} HTTP/1.1
Host: arc.taal.com
Accept: application/json

```

```javascript

const headers = {
  'Accept':'application/json',
  'Authorization':'Bearer {access-token}'
};

fetch('https://arc.taal.com/v1/tx/{txid}',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

```java
URL obj = new URL("https://arc.taal.com/v1/tx/{txid}");
HttpURLConnection con = (HttpURLConnection) obj.openConnection();
con.setRequestMethod("GET");
int responseCode = con.getResponseCode();
BufferedReader in = new BufferedReader(
    new InputStreamReader(con.getInputStream()));
String inputLine;
StringBuffer response = new StringBuffer();
while ((inputLine = in.readLine()) != null) {
    response.append(inputLine);
}
in.close();
System.out.println(response.toString());

```

```go
package main

import (
       "bytes"
       "net/http"
)

func main() {

    headers := map[string][]string{
        "Accept": []string{"application/json"},
        "Authorization": []string{"Bearer {access-token}"},
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("GET", "https://arc.taal.com/v1/tx/{txid}", data)
    req.Header = headers

    client := &http.Client{}
    resp, err := client.Do(req)
    // ...
}

```

```ruby
require 'rest-client'
require 'json'

headers = {
  'Accept' => 'application/json',
  'Authorization' => 'Bearer {access-token}'
}

result = RestClient.get 'https://arc.taal.com/v1/tx/{txid}',
  params: {
  }, headers: headers

p JSON.parse(result)

```

```python
import requests
headers = {
  'Accept': 'application/json',
  'Authorization': 'Bearer {access-token}'
}

r = requests.get('https://arc.taal.com/v1/tx/{txid}', headers = headers)

print(r.json())

```

```shell
# You can also use wget
curl -X GET https://arc.taal.com/v1/tx/{txid} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

`GET /v1/tx/{txid}`

This endpoint is used to get the current status of a previously submitted transaction.

<h3 id="get-transaction-status.-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|txid|path|string|true|The transaction ID (32 byte hash) hex string|

> Example responses

> 200 Response

```json
{
  "timestamp": "2019-08-24T14:15:22Z",
  "blockHash": "00000000000000000854749b3c125d52c6943677544c8a6a885247935ba8d17d",
  "blockHeight": 782318,
  "txid": "6bdbcfab0526d30e8d68279f79dff61fb4026ace8b7b32789af016336e54f2f0",
  "merklePath": "0000",
  "txStatus": "ACCEPTED_BY_NETWORK",
  "extraInfo": "Transaction is not valid",
  "competingTxs": [
    [
      "c0d6fce714e4225614f000c6a5addaaa1341acbb9c87115114dcf84f37b945a6"
    ]
  ]
}
```

<h3 id="get-transaction-status.-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Success|[TransactionStatus](#schematransactionstatus)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Security requirements failed|None|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|[ErrorNotFound](#schemaerrornotfound)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Generic error|[ErrorGeneric](#schemaerrorgeneric)|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
BearerAuth, None, None
</aside>

## Submit a transaction.

<a id="opIdPOST transaction"></a>

> Code samples

```http
POST https://arc.taal.com/v1/tx HTTP/1.1
Host: arc.taal.com
Content-Type: text/plain
Accept: application/json
X-CallbackUrl: string
X-FullStatusUpdates: true
X-MaxTimeout: 0
X-SkipFeeValidation: true
X-ForceValidation: true
X-SkipScriptValidation: true
X-SkipTxValidation: true
X-CumulativeFeeValidation: true
X-CallbackToken: string
X-CallbackBatch: true
X-WaitFor: string

```

```javascript
const inputBody = '<transaction hex string>';
const headers = {
  'Content-Type':'text/plain',
  'Accept':'application/json',
  'X-CallbackUrl':'string',
  'X-FullStatusUpdates':'true',
  'X-MaxTimeout':'0',
  'X-SkipFeeValidation':'true',
  'X-ForceValidation':'true',
  'X-SkipScriptValidation':'true',
  'X-SkipTxValidation':'true',
  'X-CumulativeFeeValidation':'true',
  'X-CallbackToken':'string',
  'X-CallbackBatch':'true',
  'X-WaitFor':'string',
  'Authorization':'Bearer {access-token}'
};

fetch('https://arc.taal.com/v1/tx',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

```java
URL obj = new URL("https://arc.taal.com/v1/tx");
HttpURLConnection con = (HttpURLConnection) obj.openConnection();
con.setRequestMethod("POST");
int responseCode = con.getResponseCode();
BufferedReader in = new BufferedReader(
    new InputStreamReader(con.getInputStream()));
String inputLine;
StringBuffer response = new StringBuffer();
while ((inputLine = in.readLine()) != null) {
    response.append(inputLine);
}
in.close();
System.out.println(response.toString());

```

```go
package main

import (
       "bytes"
       "net/http"
)

func main() {

    headers := map[string][]string{
        "Content-Type": []string{"text/plain"},
        "Accept": []string{"application/json"},
        "X-CallbackUrl": []string{"string"},
        "X-FullStatusUpdates": []string{"true"},
        "X-MaxTimeout": []string{"0"},
        "X-SkipFeeValidation": []string{"true"},
        "X-ForceValidation": []string{"true"},
        "X-SkipScriptValidation": []string{"true"},
        "X-SkipTxValidation": []string{"true"},
        "X-CumulativeFeeValidation": []string{"true"},
        "X-CallbackToken": []string{"string"},
        "X-CallbackBatch": []string{"true"},
        "X-WaitFor": []string{"string"},
        "Authorization": []string{"Bearer {access-token}"},
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("POST", "https://arc.taal.com/v1/tx", data)
    req.Header = headers

    client := &http.Client{}
    resp, err := client.Do(req)
    // ...
}

```

```ruby
require 'rest-client'
require 'json'

headers = {
  'Content-Type' => 'text/plain',
  'Accept' => 'application/json',
  'X-CallbackUrl' => 'string',
  'X-FullStatusUpdates' => 'true',
  'X-MaxTimeout' => '0',
  'X-SkipFeeValidation' => 'true',
  'X-ForceValidation' => 'true',
  'X-SkipScriptValidation' => 'true',
  'X-SkipTxValidation' => 'true',
  'X-CumulativeFeeValidation' => 'true',
  'X-CallbackToken' => 'string',
  'X-CallbackBatch' => 'true',
  'X-WaitFor' => 'string',
  'Authorization' => 'Bearer {access-token}'
}

result = RestClient.post 'https://arc.taal.com/v1/tx',
  params: {
  }, headers: headers

p JSON.parse(result)

```

```python
import requests
headers = {
  'Content-Type': 'text/plain',
  'Accept': 'application/json',
  'X-CallbackUrl': 'string',
  'X-FullStatusUpdates': 'true',
  'X-MaxTimeout': '0',
  'X-SkipFeeValidation': 'true',
  'X-ForceValidation': 'true',
  'X-SkipScriptValidation': 'true',
  'X-SkipTxValidation': 'true',
  'X-CumulativeFeeValidation': 'true',
  'X-CallbackToken': 'string',
  'X-CallbackBatch': 'true',
  'X-WaitFor': 'string',
  'Authorization': 'Bearer {access-token}'
}

r = requests.post('https://arc.taal.com/v1/tx', headers = headers)

print(r.json())

```

```shell
# You can also use wget
curl -X POST https://arc.taal.com/v1/tx \
  -H 'Content-Type: text/plain' \
  -H 'Accept: application/json' \
  -H 'X-CallbackUrl: string' \
  -H 'X-FullStatusUpdates: true' \
  -H 'X-MaxTimeout: 0' \
  -H 'X-SkipFeeValidation: true' \
  -H 'X-ForceValidation: true' \
  -H 'X-SkipScriptValidation: true' \
  -H 'X-SkipTxValidation: true' \
  -H 'X-CumulativeFeeValidation: true' \
  -H 'X-CallbackToken: string' \
  -H 'X-CallbackBatch: true' \
  -H 'X-WaitFor: string' \
  -H 'Authorization: Bearer {access-token}'

```

`POST /v1/tx`

This endpoint is used to send a raw transaction to a miner for inclusion in the next block that the miner creates.

> Body parameter

```json
"<transaction hex string>"
```

```
<transaction hex string>

```

```yaml
<transaction hex string>

```

<h3 id="submit-a-transaction.-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|X-CallbackUrl|header|string|false|Default double spend and merkle proof notification callback endpoint.|
|X-FullStatusUpdates|header|boolean|false|Whether we should have full status updates in callback or not (including SEEN_IN_ORPHAN_MEMPOOL and SEEN_ON_NETWORK statuses).|
|X-MaxTimeout|header|integer|false|Timeout in seconds to wait for new transaction status before request expires (max 30 seconds, default 5)|
|X-SkipFeeValidation|header|boolean|false|Whether we should skip fee validation or not.|
|X-ForceValidation|header|boolean|false|Whether we should force submitted tx validation in any case.|
|X-SkipScriptValidation|header|boolean|false|Whether we should skip script validation or not.|
|X-SkipTxValidation|header|boolean|false|Whether we should skip overall tx validation or not.|
|X-CumulativeFeeValidation|header|boolean|false|Whether we should perform cumulative fee validation for fee consolidation txs or not.|
|X-CallbackToken|header|string|false|Access token for notification callback endpoint. It will be used as a Authorization header for the http callback|
|X-CallbackBatch|header|boolean|false|Callback will be send in a batch|
|X-WaitFor|header|string|false|Which status to wait for from the server before returning ('QUEUED', 'RECEIVED', 'STORED', 'ANNOUNCED_TO_NETWORK', 'REQUESTED_BY_NETWORK', 'SENT_TO_NETWORK', 'ACCEPTED_BY_NETWORK', 'SEEN_ON_NETWORK')|
|body|body|string|true|Transaction hex string|

> Example responses

> Success

```json
{
  "blockHash": "0000000000000aac89fbed163ed60061ba33bc0ab9de8e7fd8b34ad94c2414cd",
  "blockHeight": 736228,
  "extraInfo": "",
  "merklePath": "fe54251800020400028d97f9ebeddd9f9aa8e0e953b3a76f316298ab05e9834aa811716e9d397564e501025f64aa8e012e26a5c5803c9f94d1c2c8ea68ecef1415011e1c2e26b9c966b6ad02021f5fa39607ca3b48d53c902bd5bb4bbf6a7ac99cf9fda45cc21b71e6e2f7889603024a2bb116e86325c9b8512f10b22c228ab3272fe3f373b1bd4a9a6b334b068bb602000061793b278303101a1390ceae5a713de0eabd9cda63702fe84c928970acf7c45e0100a567e3d066e38638b27897559302eabc85eb69b202c2e86d4338bab73008f460",
  "status": 200,
  "timestamp": "2023-03-09T12:03:48.382910514Z",
  "title": "OK",
  "txStatus": "MINED",
  "txid": "b68b064b336b9a4abdb173f3e32f27b38a222cb2102f51b8c92563e816b12b4a"
}
```

```json
{
  "blockHash": "",
  "blockHeight": 0,
  "extraInfo": "",
  "status": 200,
  "timestamp": "2023-03-09T12:03:48.382910514Z",
  "title": "OK",
  "txStatus": "SEEN_ON_NETWORK",
  "txid": "c0d6fce714e4225614f000c6a5addaaa1341acbb9c87115114dcf84f37b945a6",
  "merklePath": ""
}
```

```json
{
  "detail": "Transaction is invalid because the outputs are non-existent or invalid",
  "extraInfo": "arc error 463: arc error 463: transaction output 0 satoshis is invalid",
  "instance": null,
  "status": 463,
  "title": "Invalid outputs",
  "txid": "a0d69a2dfad710770ed282cce316c5792f6101a68046a263a17a1ae02676015e",
  "type": "https://bitcoin-sv.github.io/arc/#/errors?id=_463\""
}
```

> 400 Response

```json
{
  "type": "https://bitcoin-sv.github.io/arc/#/errors?id=_400",
  "title": "Bad request",
  "status": 400,
  "detail": "The request seems to be malformed and cannot be processed",
  "instance": "https://arc.taal.com/errors/1234556",
  "txid": "string",
  "extraInfo": "string"
}
```

<h3 id="submit-a-transaction.-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Success|[TransactionResponse](#schematransactionresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|[ErrorBadRequest](#schemaerrorbadrequest)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Security requirements failed|None|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Generic error|[ErrorGeneric](#schemaerrorgeneric)|
|422|[Unprocessable Entity](https://tools.ietf.org/html/rfc2518#section-10.3)|Unprocessable entity - with IETF RFC 7807 Error object|[Error](#schemaerror)|
|460|Unknown|Not extended format|[ErrorTxFormat](#schemaerrortxformat)|
|461|Unknown|Malformed transaction|[ErrorUnlockingScripts](#schemaerrorunlockingscripts)|
|462|Unknown|Invalid inputs|[ErrorInputs](#schemaerrorinputs)|
|463|Unknown|Malformed transaction|[ErrorMalformed](#schemaerrormalformed)|
|464|Unknown|Invalid outputs|[ErrorOutputs](#schemaerroroutputs)|
|465|Unknown|Fee too low|[ErrorFee](#schemaerrorfee)|
|467|Unknown|Mined ancestors not found in BEEF|[ErrorMinedAncestorsNotFound](#schemaerrorminedancestorsnotfound)|
|468|Unknown|Invalid BUMPs in BEEF|[ErrorCalculatingMerkleRoots](#schemaerrorcalculatingmerkleroots)|
|469|Unknown|Invalid Merkle Roots|[ErrorValidatingMerkleRoots](#schemaerrorvalidatingmerkleroots)|
|473|Unknown|Cumulative Fee validation failed|[ErrorCumulativeFees](#schemaerrorcumulativefees)|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
BearerAuth, None, None
</aside>

## Submit multiple transactions.

<a id="opIdPOST transactions"></a>

> Code samples

```http
POST https://arc.taal.com/v1/txs HTTP/1.1
Host: arc.taal.com
Content-Type: text/plain
Accept: application/json
X-CallbackUrl: string
X-FullStatusUpdates: true
X-MaxTimeout: 0
X-SkipFeeValidation: true
X-ForceValidation: true
X-SkipScriptValidation: true
X-SkipTxValidation: true
X-CumulativeFeeValidation: true
X-CallbackToken: string
X-CallbackBatch: true
X-WaitFor: string

```

```javascript
const inputBody = '<transaction hex string>
<transaction hex string>';
const headers = {
  'Content-Type':'text/plain',
  'Accept':'application/json',
  'X-CallbackUrl':'string',
  'X-FullStatusUpdates':'true',
  'X-MaxTimeout':'0',
  'X-SkipFeeValidation':'true',
  'X-ForceValidation':'true',
  'X-SkipScriptValidation':'true',
  'X-SkipTxValidation':'true',
  'X-CumulativeFeeValidation':'true',
  'X-CallbackToken':'string',
  'X-CallbackBatch':'true',
  'X-WaitFor':'string',
  'Authorization':'Bearer {access-token}'
};

fetch('https://arc.taal.com/v1/txs',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

```java
URL obj = new URL("https://arc.taal.com/v1/txs");
HttpURLConnection con = (HttpURLConnection) obj.openConnection();
con.setRequestMethod("POST");
int responseCode = con.getResponseCode();
BufferedReader in = new BufferedReader(
    new InputStreamReader(con.getInputStream()));
String inputLine;
StringBuffer response = new StringBuffer();
while ((inputLine = in.readLine()) != null) {
    response.append(inputLine);
}
in.close();
System.out.println(response.toString());

```

```go
package main

import (
       "bytes"
       "net/http"
)

func main() {

    headers := map[string][]string{
        "Content-Type": []string{"text/plain"},
        "Accept": []string{"application/json"},
        "X-CallbackUrl": []string{"string"},
        "X-FullStatusUpdates": []string{"true"},
        "X-MaxTimeout": []string{"0"},
        "X-SkipFeeValidation": []string{"true"},
        "X-ForceValidation": []string{"true"},
        "X-SkipScriptValidation": []string{"true"},
        "X-SkipTxValidation": []string{"true"},
        "X-CumulativeFeeValidation": []string{"true"},
        "X-CallbackToken": []string{"string"},
        "X-CallbackBatch": []string{"true"},
        "X-WaitFor": []string{"string"},
        "Authorization": []string{"Bearer {access-token}"},
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("POST", "https://arc.taal.com/v1/txs", data)
    req.Header = headers

    client := &http.Client{}
    resp, err := client.Do(req)
    // ...
}

```

```ruby
require 'rest-client'
require 'json'

headers = {
  'Content-Type' => 'text/plain',
  'Accept' => 'application/json',
  'X-CallbackUrl' => 'string',
  'X-FullStatusUpdates' => 'true',
  'X-MaxTimeout' => '0',
  'X-SkipFeeValidation' => 'true',
  'X-ForceValidation' => 'true',
  'X-SkipScriptValidation' => 'true',
  'X-SkipTxValidation' => 'true',
  'X-CumulativeFeeValidation' => 'true',
  'X-CallbackToken' => 'string',
  'X-CallbackBatch' => 'true',
  'X-WaitFor' => 'string',
  'Authorization' => 'Bearer {access-token}'
}

result = RestClient.post 'https://arc.taal.com/v1/txs',
  params: {
  }, headers: headers

p JSON.parse(result)

```

```python
import requests
headers = {
  'Content-Type': 'text/plain',
  'Accept': 'application/json',
  'X-CallbackUrl': 'string',
  'X-FullStatusUpdates': 'true',
  'X-MaxTimeout': '0',
  'X-SkipFeeValidation': 'true',
  'X-ForceValidation': 'true',
  'X-SkipScriptValidation': 'true',
  'X-SkipTxValidation': 'true',
  'X-CumulativeFeeValidation': 'true',
  'X-CallbackToken': 'string',
  'X-CallbackBatch': 'true',
  'X-WaitFor': 'string',
  'Authorization': 'Bearer {access-token}'
}

r = requests.post('https://arc.taal.com/v1/txs', headers = headers)

print(r.json())

```

```shell
# You can also use wget
curl -X POST https://arc.taal.com/v1/txs \
  -H 'Content-Type: text/plain' \
  -H 'Accept: application/json' \
  -H 'X-CallbackUrl: string' \
  -H 'X-FullStatusUpdates: true' \
  -H 'X-MaxTimeout: 0' \
  -H 'X-SkipFeeValidation: true' \
  -H 'X-ForceValidation: true' \
  -H 'X-SkipScriptValidation: true' \
  -H 'X-SkipTxValidation: true' \
  -H 'X-CumulativeFeeValidation: true' \
  -H 'X-CallbackToken: string' \
  -H 'X-CallbackBatch: true' \
  -H 'X-WaitFor: string' \
  -H 'Authorization: Bearer {access-token}'

```

`POST /v1/txs`

This endpoint is used to send multiple raw transactions to a miner for inclusion in the next block that the miner creates.

> Body parameter

```json
"<transaction hex string>\n<transaction hex string>"
```

```
|-
<transaction hex string>
<transaction hex string>

```

```yaml
|-
<transaction hex string>
<transaction hex string>

```

<h3 id="submit-multiple-transactions.-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|X-CallbackUrl|header|string|false|Default double spend and merkle proof notification callback endpoint.|
|X-FullStatusUpdates|header|boolean|false|Whether we should have full status updates in callback or not (including SEEN_IN_ORPHAN_MEMPOOL and SEEN_ON_NETWORK statuses).|
|X-MaxTimeout|header|integer|false|Timeout in seconds to wait for new transaction status before request expires (max 30 seconds, default 5)|
|X-SkipFeeValidation|header|boolean|false|Whether we should skip fee validation or not.|
|X-ForceValidation|header|boolean|false|Whether we should force submitted tx validation in any case.|
|X-SkipScriptValidation|header|boolean|false|Whether we should skip script validation or not.|
|X-SkipTxValidation|header|boolean|false|Whether we should skip overall tx validation or not.|
|X-CumulativeFeeValidation|header|boolean|false|Whether we should perform cumulative fee validation for fee consolidation txs or not.|
|X-CallbackToken|header|string|false|Access token for notification callback endpoint. It will be used as a Authorization header for the http callback|
|X-CallbackBatch|header|boolean|false|Callback will be send in a batch|
|X-WaitFor|header|string|false|Which status to wait for from the server before returning ('QUEUED', 'RECEIVED', 'STORED', 'ANNOUNCED_TO_NETWORK', 'REQUESTED_BY_NETWORK', 'SENT_TO_NETWORK', 'ACCEPTED_BY_NETWORK', 'SEEN_ON_NETWORK')|
|body|body|string|false|none|

> Example responses

> Transaction status

```json
[
  {
    "blockHash": "0000000000000aac89fbed163ed60061ba33bc0ab9de8e7fd8b34ad94c2414cd",
    "blockHeight": 761868,
    "extraInfo": "",
    "status": 200,
    "timestamp": "2023-03-09T12:03:48.382910514Z",
    "title": "OK",
    "txStatus": "MINED",
    "txid": "b68b064b336b9a4abdb173f3e32f27b38a222cb2102f51b8c92563e816b12b4a",
    "merklePath": "fe54251800020400028d97f9ebeddd9f9aa8e0e953b3a76f316298ab05e9834aa811716e9d397564e501025f64aa8e012e26a5c5803c9f94d1c2c8ea68ecef1415011e1c2e26b9c966b6ad02021f5fa39607ca3b48d53c902bd5bb4bbf6a7ac99cf9fda45cc21b71e6e2f7889603024a2bb116e86325c9b8512f10b22c228ab3272fe3f373b1bd4a9a6b334b068bb602000061793b278303101a1390ceae5a713de0eabd9cda63702fe84c928970acf7c45e0100a567e3d066e38638b27897559302eabc85eb69b202c2e86d4338bab73008f460"
  }
]
```

```json
[
  {
    "blockHash": "",
    "blockHeight": 0,
    "extraInfo": "",
    "status": 200,
    "timestamp": "2023-03-09T12:03:48.382910514Z",
    "title": "OK",
    "txStatus": "SEEN_ON_NETWORK",
    "txid": "c0d6fce714e4225614f000c6a5addaaa1341acbb9c87115114dcf84f37b945a6",
    "merklePath": ""
  }
]
```

```json
[
  {
    "detail": "Transaction is invalid because the outputs are non-existent or invalid",
    "extraInfo": "arc error 463: arc error 463: transaction output 0 satoshis is invalid",
    "instance": null,
    "status": 463,
    "title": "Invalid outputs",
    "txid": "a0d69a2dfad710770ed282cce316c5792f6101a68046a263a17a1ae02676015e",
    "type": "https://bitcoin-sv.github.io/arc/#/errors?id=_463"
  }
]
```

> 400 Response

```json
{
  "type": "https://bitcoin-sv.github.io/arc/#/errors?id=_400",
  "title": "Bad request",
  "status": 400,
  "detail": "The request seems to be malformed and cannot be processed",
  "instance": "https://arc.taal.com/errors/1234556",
  "txid": "string",
  "extraInfo": "string"
}
```

<h3 id="submit-multiple-transactions.-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Transaction status|[TransactionResponses](#schematransactionresponses)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|[ErrorBadRequest](#schemaerrorbadrequest)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Security requirements failed|None|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Generic error|[ErrorGeneric](#schemaerrorgeneric)|
|460|Unknown|Not extended format|[ErrorTxFormat](#schemaerrortxformat)|
|461|Unknown|Malformed transaction|[ErrorUnlockingScripts](#schemaerrorunlockingscripts)|
|462|Unknown|Invalid inputs|[ErrorInputs](#schemaerrorinputs)|
|463|Unknown|Malformed transaction|[ErrorMalformed](#schemaerrormalformed)|
|464|Unknown|Invalid outputs|[ErrorOutputs](#schemaerroroutputs)|
|465|Unknown|Fee too low|[ErrorFee](#schemaerrorfee)|
|467|Unknown|Mined ancestors not found in BEEF|[ErrorMinedAncestorsNotFound](#schemaerrorminedancestorsnotfound)|
|468|Unknown|Invalid BUMPs in BEEF|[ErrorCalculatingMerkleRoots](#schemaerrorcalculatingmerkleroots)|
|469|Unknown|Invalid Merkle Roots|[ErrorValidatingMerkleRoots](#schemaerrorvalidatingmerkleroots)|
|473|Unknown|Cumulative Fee too low|[ErrorCumulativeFees](#schemaerrorcumulativefees)|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
BearerAuth, None, None
</aside>

# Schemas

<h2 id="tocS_CommonResponse">CommonResponse</h2>
<!-- backwards compatibility -->
<a id="schemacommonresponse"></a>
<a id="schema_CommonResponse"></a>
<a id="tocScommonresponse"></a>
<a id="tocscommonresponse"></a>

```json
{
  "timestamp": "2019-08-24T14:15:22Z"
}

```

Common response object

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|timestamp|string(date-time)|true|none|none|

<h2 id="tocS_Health">Health</h2>
<!-- backwards compatibility -->
<a id="schemahealth"></a>
<a id="schema_Health"></a>
<a id="tocShealth"></a>
<a id="tocshealth"></a>

```json
{
  "healthy": false,
  "version": "v1.0.0",
  "reason": "no db connection"
}

```

healthy or not

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|healthy|boolean|false|none|whether healthy or not|
|version|string|false|none|version of the ARC package|
|reason|string¦null|false|none|explains the problem with metamorph|

<h2 id="tocS_ChainInfo">ChainInfo</h2>
<!-- backwards compatibility -->
<a id="schemachaininfo"></a>
<a id="schema_ChainInfo"></a>
<a id="tocSchaininfo"></a>
<a id="tocschaininfo"></a>

```json
{
  "blockHash": "00000000000000000854749b3c125d52c6943677544c8a6a885247935ba8d17d",
  "blockHeight": 782318
}

```

Chain info

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|blockHash|string|false|none|Block hash|
|blockHeight|integer(uint64)|false|none|Block height|

<h2 id="tocS_PolicyResponse">PolicyResponse</h2>
<!-- backwards compatibility -->
<a id="schemapolicyresponse"></a>
<a id="schema_PolicyResponse"></a>
<a id="tocSpolicyresponse"></a>
<a id="tocspolicyresponse"></a>

```json
{
  "timestamp": "2019-08-24T14:15:22Z",
  "policy": {
    "maxscriptsizepolicy": 500000,
    "maxtxsigopscountspolicy": 4294967295,
    "maxtxsizepolicy": 10000000,
    "miningFee": {
      "satoshis": 1,
      "bytes": 1000
    }
  }
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[CommonResponse](#schemacommonresponse)|false|none|Common response object|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» policy|[Policy](#schemapolicy)|true|none|none|

<h2 id="tocS_Policy">Policy</h2>
<!-- backwards compatibility -->
<a id="schemapolicy"></a>
<a id="schema_Policy"></a>
<a id="tocSpolicy"></a>
<a id="tocspolicy"></a>

```json
{
  "maxscriptsizepolicy": 500000,
  "maxtxsigopscountspolicy": 4294967295,
  "maxtxsizepolicy": 10000000,
  "miningFee": {
    "satoshis": 1,
    "bytes": 1000
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|maxscriptsizepolicy|integer(uint64)|true|none|Maximum script size [bytes]|
|maxtxsigopscountspolicy|integer(uint64)|true|none|Maximum number of signature operations|
|maxtxsizepolicy|integer(uint64)|true|none|Maximum transaction size [bytes]|
|miningFee|[FeeAmount](#schemafeeamount)|true|none|Mining fee|

<h2 id="tocS_FeeAmount">FeeAmount</h2>
<!-- backwards compatibility -->
<a id="schemafeeamount"></a>
<a id="schema_FeeAmount"></a>
<a id="tocSfeeamount"></a>
<a id="tocsfeeamount"></a>

```json
{
  "satoshis": 1,
  "bytes": 1000
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|satoshis|integer(uint64)|true|none|Amount in Satoshis|
|bytes|integer(uint64)|true|none|Number of bytes|

<h2 id="tocS_TransactionRequest">TransactionRequest</h2>
<!-- backwards compatibility -->
<a id="schematransactionrequest"></a>
<a id="schema_TransactionRequest"></a>
<a id="tocStransactionrequest"></a>
<a id="tocstransactionrequest"></a>

```json
{
  "rawTx": "<transaction hex string>"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|rawTx|string|true|none|Raw hex string|

<h2 id="tocS_TransactionResponse">TransactionResponse</h2>
<!-- backwards compatibility -->
<a id="schematransactionresponse"></a>
<a id="schema_TransactionResponse"></a>
<a id="tocStransactionresponse"></a>
<a id="tocstransactionresponse"></a>

```json
{
  "timestamp": "2019-08-24T14:15:22Z",
  "blockHash": "00000000000000000854749b3c125d52c6943677544c8a6a885247935ba8d17d",
  "blockHeight": 782318,
  "txid": "6bdbcfab0526d30e8d68279f79dff61fb4026ace8b7b32789af016336e54f2f0",
  "merklePath": "0000",
  "txStatus": "ACCEPTED_BY_NETWORK",
  "extraInfo": "Transaction is not valid",
  "competingTxs": [
    [
      "c0d6fce714e4225614f000c6a5addaaa1341acbb9c87115114dcf84f37b945a6"
    ]
  ],
  "status": 201,
  "title": "Added to mempool"
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[CommonResponse](#schemacommonresponse)|false|none|Common response object|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ChainInfo](#schemachaininfo)|false|none|Chain info|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[TransactionDetails](#schematransactiondetails)|false|none|Transaction details|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[TransactionSubmitStatus](#schematransactionsubmitstatus)|false|none|Transaction submit status|

<h2 id="tocS_TransactionResponses">TransactionResponses</h2>
<!-- backwards compatibility -->
<a id="schematransactionresponses"></a>
<a id="schema_TransactionResponses"></a>
<a id="tocStransactionresponses"></a>
<a id="tocstransactionresponses"></a>

```json
{
  "transactions": [
    {
      "timestamp": "2019-08-24T14:15:22Z",
      "blockHash": "00000000000000000854749b3c125d52c6943677544c8a6a885247935ba8d17d",
      "blockHeight": 782318,
      "txid": "6bdbcfab0526d30e8d68279f79dff61fb4026ace8b7b32789af016336e54f2f0",
      "merklePath": "0000",
      "txStatus": "ACCEPTED_BY_NETWORK",
      "extraInfo": "Transaction is not valid",
      "competingTxs": [
        [
          "c0d6fce714e4225614f000c6a5addaaa1341acbb9c87115114dcf84f37b945a6"
        ]
      ],
      "status": 201,
      "title": "Added to mempool"
    }
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|transactions|[oneOf]|false|none|none|

oneOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» *anonymous*|[TransactionResponse](#schematransactionresponse)|false|none|none|

xor

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» *anonymous*|[Error](#schemaerror)|false|none|An HTTP Problem Details object, as defined in IETF RFC 7807 (https://tools.ietf.org/html/rfc7807).|

<h2 id="tocS_TransactionStatus">TransactionStatus</h2>
<!-- backwards compatibility -->
<a id="schematransactionstatus"></a>
<a id="schema_TransactionStatus"></a>
<a id="tocStransactionstatus"></a>
<a id="tocstransactionstatus"></a>

```json
{
  "timestamp": "2019-08-24T14:15:22Z",
  "blockHash": "00000000000000000854749b3c125d52c6943677544c8a6a885247935ba8d17d",
  "blockHeight": 782318,
  "txid": "6bdbcfab0526d30e8d68279f79dff61fb4026ace8b7b32789af016336e54f2f0",
  "merklePath": "0000",
  "txStatus": "ACCEPTED_BY_NETWORK",
  "extraInfo": "Transaction is not valid",
  "competingTxs": [
    [
      "c0d6fce714e4225614f000c6a5addaaa1341acbb9c87115114dcf84f37b945a6"
    ]
  ]
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[CommonResponse](#schemacommonresponse)|false|none|Common response object|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ChainInfo](#schemachaininfo)|false|none|Chain info|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[TransactionDetails](#schematransactiondetails)|false|none|Transaction details|

<h2 id="tocS_TransactionSubmitStatus">TransactionSubmitStatus</h2>
<!-- backwards compatibility -->
<a id="schematransactionsubmitstatus"></a>
<a id="schema_TransactionSubmitStatus"></a>
<a id="tocStransactionsubmitstatus"></a>
<a id="tocstransactionsubmitstatus"></a>

```json
{
  "status": 201,
  "title": "Added to mempool"
}

```

Transaction submit status

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|integer(int)|true|none|Status|
|title|string|true|none|Title|

<h2 id="tocS_TransactionDetails">TransactionDetails</h2>
<!-- backwards compatibility -->
<a id="schematransactiondetails"></a>
<a id="schema_TransactionDetails"></a>
<a id="tocStransactiondetails"></a>
<a id="tocstransactiondetails"></a>

```json
{
  "txid": "6bdbcfab0526d30e8d68279f79dff61fb4026ace8b7b32789af016336e54f2f0",
  "merklePath": "0000",
  "txStatus": "ACCEPTED_BY_NETWORK",
  "extraInfo": "Transaction is not valid",
  "competingTxs": [
    [
      "c0d6fce714e4225614f000c6a5addaaa1341acbb9c87115114dcf84f37b945a6"
    ]
  ]
}

```

Transaction details

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|txid|string|true|none|Transaction ID in hex|
|merklePath|string¦null|false|none|Transaction Merkle path as a hex string in BUMP format [BRC-74](https://brc.dev/74)|
|txStatus|string|true|none|Transaction status|
|extraInfo|string¦null|false|none|Extra information about the transaction|
|competingTxs|[string]¦null|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|txStatus|UNKNOWN|
|txStatus|QUEUED|
|txStatus|RECEIVED|
|txStatus|STORED|
|txStatus|ANNOUNCED_TO_NETWORK|
|txStatus|REQUESTED_BY_NETWORK|
|txStatus|SENT_TO_NETWORK|
|txStatus|ACCEPTED_BY_NETWORK|
|txStatus|SEEN_IN_ORPHAN_MEMPOOL|
|txStatus|SEEN_ON_NETWORK|
|txStatus|DOUBLE_SPEND_ATTEMPTED|
|txStatus|REJECTED|
|txStatus|MINED|

<h2 id="tocS_Error">Error</h2>
<!-- backwards compatibility -->
<a id="schemaerror"></a>
<a id="schema_Error"></a>
<a id="tocSerror"></a>
<a id="tocserror"></a>

```json
{
  "type": "https://bitcoin-sv.github.io/arc/#/errors?id=_461",
  "title": "Malformed transaction",
  "status": 461,
  "detail": "Transaction is malformed and cannot be processed",
  "instance": "https://arc.taal.com/errors/123452",
  "txid": "string",
  "extraInfo": "string"
}

```

An HTTP Problem Details object, as defined in IETF RFC 7807 (https://tools.ietf.org/html/rfc7807).

### Properties

oneOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorUnlockingScripts](#schemaerrorunlockingscripts)|false|none|none|

xor

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorInputs](#schemaerrorinputs)|false|none|none|

xor

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorMalformed](#schemaerrormalformed)|false|none|none|

xor

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorMinedAncestorsNotFound](#schemaerrorminedancestorsnotfound)|false|none|none|

xor

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorCalculatingMerkleRoots](#schemaerrorcalculatingmerkleroots)|false|none|none|

xor

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorFrozenPolicy](#schemaerrorfrozenpolicy)|false|none|none|

xor

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorFrozenConsensus](#schemaerrorfrozenconsensus)|false|none|none|

<h2 id="tocS_ErrorFields">ErrorFields</h2>
<!-- backwards compatibility -->
<a id="schemaerrorfields"></a>
<a id="schema_ErrorFields"></a>
<a id="tocSerrorfields"></a>
<a id="tocserrorfields"></a>

```json
{
  "type": "string",
  "title": "string",
  "status": 402,
  "detail": "The fee in the transaction is too low to be included in a block.",
  "instance": "string",
  "txid": "string",
  "extraInfo": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|type|string|true|none|Type of error, with link to more information|
|title|string|true|none|Descriptive text for error|
|status|integer(int)|true|none|Error code|
|detail|string|true|none|Longer description of error|
|instance|string¦null|false|none|(Optional) Link to actual error on server|
|txid|string¦null|false|none|Transaction ID this error is referring to|
|extraInfo|string¦null|false|none|Optional extra information about the error from the miner|

<h2 id="tocS_ErrorBadRequest">ErrorBadRequest</h2>
<!-- backwards compatibility -->
<a id="schemaerrorbadrequest"></a>
<a id="schema_ErrorBadRequest"></a>
<a id="tocSerrorbadrequest"></a>
<a id="tocserrorbadrequest"></a>

```json
{
  "type": "https://bitcoin-sv.github.io/arc/#/errors?id=_400",
  "title": "Bad request",
  "status": 400,
  "detail": "The request seems to be malformed and cannot be processed",
  "instance": "https://arc.taal.com/errors/1234556",
  "txid": "string",
  "extraInfo": "string"
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorFields](#schemaerrorfields)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» type|any|false|none|none|
|» title|any|false|none|none|
|» status|any|false|none|none|
|» detail|any|false|none|none|
|» instance|any|false|none|none|

<h2 id="tocS_ErrorNotFound">ErrorNotFound</h2>
<!-- backwards compatibility -->
<a id="schemaerrornotfound"></a>
<a id="schema_ErrorNotFound"></a>
<a id="tocSerrornotfound"></a>
<a id="tocserrornotfound"></a>

```json
{
  "type": "https://bitcoin-sv.github.io/arc/#/errors?id=_404",
  "title": "Not found",
  "status": 404,
  "detail": "The requested resource could not be found",
  "instance": "https://arc.taal.com/errors/1234556",
  "txid": "string",
  "extraInfo": "string"
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorFields](#schemaerrorfields)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» type|any|false|none|none|
|» title|any|false|none|none|
|» status|any|false|none|none|
|» detail|any|false|none|none|
|» instance|any|false|none|none|

<h2 id="tocS_ErrorGeneric">ErrorGeneric</h2>
<!-- backwards compatibility -->
<a id="schemaerrorgeneric"></a>
<a id="schema_ErrorGeneric"></a>
<a id="tocSerrorgeneric"></a>
<a id="tocserrorgeneric"></a>

```json
{
  "type": "https://bitcoin-sv.github.io/arc/#/errors?id=_409",
  "title": "Generic error",
  "status": 409,
  "detail": "Transaction could not be processed",
  "instance": "https://arc.taal.com/errors/1234556",
  "txid": "string",
  "extraInfo": "string"
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorFields](#schemaerrorfields)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» type|any|false|none|none|
|» title|any|false|none|none|
|» status|any|false|none|none|
|» detail|any|false|none|none|
|» instance|any|false|none|none|

<h2 id="tocS_ErrorTxFormat">ErrorTxFormat</h2>
<!-- backwards compatibility -->
<a id="schemaerrortxformat"></a>
<a id="schema_ErrorTxFormat"></a>
<a id="tocSerrortxformat"></a>
<a id="tocserrortxformat"></a>

```json
{
  "type": "https://bitcoin-sv.github.io/arc/#/errors?id=_460",
  "title": "Not extended format",
  "status": 460,
  "detail": "Missing input scripts: Transaction could not be transformed to extended format",
  "instance": "https://arc.taal.com/errors/1234556",
  "txid": "string",
  "extraInfo": "string"
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorFields](#schemaerrorfields)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» type|any|false|none|none|
|» title|any|false|none|none|
|» status|any|false|none|none|
|» detail|any|false|none|none|
|» instance|any|false|none|none|

<h2 id="tocS_ErrorUnlockingScripts">ErrorUnlockingScripts</h2>
<!-- backwards compatibility -->
<a id="schemaerrorunlockingscripts"></a>
<a id="schema_ErrorUnlockingScripts"></a>
<a id="tocSerrorunlockingscripts"></a>
<a id="tocserrorunlockingscripts"></a>

```json
{
  "type": "https://bitcoin-sv.github.io/arc/#/errors?id=_461",
  "title": "Malformed transaction",
  "status": 461,
  "detail": "Transaction is malformed and cannot be processed",
  "instance": "https://arc.taal.com/errors/123452",
  "txid": "string",
  "extraInfo": "string"
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorFields](#schemaerrorfields)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» type|any|false|none|none|
|» title|any|false|none|none|
|» status|any|false|none|none|
|» detail|any|false|none|none|
|» instance|any|false|none|none|

<h2 id="tocS_ErrorInputs">ErrorInputs</h2>
<!-- backwards compatibility -->
<a id="schemaerrorinputs"></a>
<a id="schema_ErrorInputs"></a>
<a id="tocSerrorinputs"></a>
<a id="tocserrorinputs"></a>

```json
{
  "type": "https://bitcoin-sv.github.io/arc/#/errors?id=_462",
  "title": "Invalid inputs",
  "status": 462,
  "detail": "Transaction is invalid because the inputs are non-existent or spent",
  "instance": "https://arc.taal.com/errors/123452",
  "txid": "string",
  "extraInfo": "string"
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorFields](#schemaerrorfields)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» type|any|false|none|none|
|» title|any|false|none|none|
|» status|any|false|none|none|
|» detail|any|false|none|none|
|» instance|any|false|none|none|

<h2 id="tocS_ErrorMalformed">ErrorMalformed</h2>
<!-- backwards compatibility -->
<a id="schemaerrormalformed"></a>
<a id="schema_ErrorMalformed"></a>
<a id="tocSerrormalformed"></a>
<a id="tocserrormalformed"></a>

```json
{
  "type": "https://bitcoin-sv.github.io/arc/#/errors?id=_463",
  "title": "Malformed transaction",
  "status": 463,
  "detail": "Transaction is malformed and cannot be processed",
  "instance": "https://arc.taal.com/errors/123452",
  "txid": "string",
  "extraInfo": "string"
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorFields](#schemaerrorfields)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» type|any|false|none|none|
|» title|any|false|none|none|
|» status|any|false|none|none|
|» detail|any|false|none|none|
|» instance|any|false|none|none|

<h2 id="tocS_ErrorOutputs">ErrorOutputs</h2>
<!-- backwards compatibility -->
<a id="schemaerroroutputs"></a>
<a id="schema_ErrorOutputs"></a>
<a id="tocSerroroutputs"></a>
<a id="tocserroroutputs"></a>

```json
{
  "type": "https://bitcoin-sv.github.io/arc/#/errors?id=_464",
  "title": "Invalid outputs",
  "status": 464,
  "detail": "Transaction is invalid because the outputs are non-existent or invalid",
  "instance": "https://arc.taal.com/errors/123452",
  "txid": "string",
  "extraInfo": "string"
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorFields](#schemaerrorfields)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» type|any|false|none|none|
|» title|any|false|none|none|
|» status|any|false|none|none|
|» detail|any|false|none|none|
|» instance|any|false|none|none|

<h2 id="tocS_ErrorFee">ErrorFee</h2>
<!-- backwards compatibility -->
<a id="schemaerrorfee"></a>
<a id="schema_ErrorFee"></a>
<a id="tocSerrorfee"></a>
<a id="tocserrorfee"></a>

```json
{
  "type": "https://bitcoin-sv.github.io/arc/#/errors?id=_465",
  "title": "Fee too low",
  "status": 465,
  "detail": "The fees are too low",
  "instance": "https://arc.taal.com/errors/123452",
  "txid": "string",
  "extraInfo": "string"
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorFields](#schemaerrorfields)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» type|any|false|none|none|
|» title|any|false|none|none|
|» status|any|false|none|none|
|» detail|any|false|none|none|
|» instance|any|false|none|none|

<h2 id="tocS_ErrorConflict">ErrorConflict</h2>
<!-- backwards compatibility -->
<a id="schemaerrorconflict"></a>
<a id="schema_ErrorConflict"></a>
<a id="tocSerrorconflict"></a>
<a id="tocserrorconflict"></a>

```json
{
  "type": "https://bitcoin-sv.github.io/arc/#/errors?id=_466",
  "title": "Conflicting tx found",
  "status": 466,
  "detail": "Transaction is valid, but there is a conflicting tx in the block template",
  "instance": "https://arc.taal.com/errors/123453",
  "txid": "string",
  "extraInfo": "string"
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorFields](#schemaerrorfields)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» type|any|false|none|none|
|» title|any|false|none|none|
|» status|any|false|none|none|
|» detail|any|false|none|none|
|» instance|any|false|none|none|

<h2 id="tocS_ErrorMinedAncestorsNotFound">ErrorMinedAncestorsNotFound</h2>
<!-- backwards compatibility -->
<a id="schemaerrorminedancestorsnotfound"></a>
<a id="schema_ErrorMinedAncestorsNotFound"></a>
<a id="tocSerrorminedancestorsnotfound"></a>
<a id="tocserrorminedancestorsnotfound"></a>

```json
{
  "type": "https://bitcoin-sv.github.io/arc/#/errors?id=_467",
  "title": "Mined ancestors not found",
  "status": 467,
  "detail": "Error validating BEEF: mined ancestors not found in transaction inputs",
  "instance": "https://arc.taal.com/errors/123453",
  "txid": "string",
  "extraInfo": "string"
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorFields](#schemaerrorfields)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» type|any|false|none|none|
|» title|any|false|none|none|
|» status|any|false|none|none|
|» detail|any|false|none|none|
|» instance|any|false|none|none|

<h2 id="tocS_ErrorCalculatingMerkleRoots">ErrorCalculatingMerkleRoots</h2>
<!-- backwards compatibility -->
<a id="schemaerrorcalculatingmerkleroots"></a>
<a id="schema_ErrorCalculatingMerkleRoots"></a>
<a id="tocSerrorcalculatingmerkleroots"></a>
<a id="tocserrorcalculatingmerkleroots"></a>

```json
{
  "type": "https://bitcoin-sv.github.io/arc/#/errors?id=_468",
  "title": "Invalid BUMPs",
  "status": 468,
  "detail": "Error validating BEEF: could not calculate Merkle Roots from given BUMPs",
  "instance": "https://arc.taal.com/errors/123453",
  "txid": "string",
  "extraInfo": "string"
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorFields](#schemaerrorfields)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» type|any|false|none|none|
|» title|any|false|none|none|
|» status|any|false|none|none|
|» detail|any|false|none|none|
|» instance|any|false|none|none|

<h2 id="tocS_ErrorValidatingMerkleRoots">ErrorValidatingMerkleRoots</h2>
<!-- backwards compatibility -->
<a id="schemaerrorvalidatingmerkleroots"></a>
<a id="schema_ErrorValidatingMerkleRoots"></a>
<a id="tocSerrorvalidatingmerkleroots"></a>
<a id="tocserrorvalidatingmerkleroots"></a>

```json
{
  "type": "https://bitcoin-sv.github.io/arc/#/errors?id=_469",
  "title": "Merkle Roots validation failed",
  "status": 469,
  "detail": "Error validating BEEF: could not validate Merkle Roots",
  "instance": "https://arc.taal.com/errors/123453",
  "txid": "string",
  "extraInfo": "string"
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorFields](#schemaerrorfields)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» type|any|false|none|none|
|» title|any|false|none|none|
|» status|any|false|none|none|
|» detail|any|false|none|none|
|» instance|any|false|none|none|

<h2 id="tocS_ErrorFrozenPolicy">ErrorFrozenPolicy</h2>
<!-- backwards compatibility -->
<a id="schemaerrorfrozenpolicy"></a>
<a id="schema_ErrorFrozenPolicy"></a>
<a id="tocSerrorfrozenpolicy"></a>
<a id="tocserrorfrozenpolicy"></a>

```json
{
  "type": "https://bitcoin-sv.github.io/arc/#/errors?id=_471",
  "title": "Input Frozen",
  "status": 471,
  "detail": "Input Frozen (blacklist manager policy blacklisted)",
  "instance": "https://arc.taal.com/errors/123452",
  "txid": "string",
  "extraInfo": "string"
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorFields](#schemaerrorfields)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» type|any|false|none|none|
|» title|any|false|none|none|
|» status|any|false|none|none|
|» detail|any|false|none|none|
|» instance|any|false|none|none|

<h2 id="tocS_ErrorFrozenConsensus">ErrorFrozenConsensus</h2>
<!-- backwards compatibility -->
<a id="schemaerrorfrozenconsensus"></a>
<a id="schema_ErrorFrozenConsensus"></a>
<a id="tocSerrorfrozenconsensus"></a>
<a id="tocserrorfrozenconsensus"></a>

```json
{
  "type": "https://bitcoin-sv.github.io/arc/#/errors?id=_472",
  "title": "Input Frozen",
  "status": 472,
  "detail": "Input Frozen (blacklist manager consensus blacklisted)",
  "instance": "https://arc.taal.com/errors/123452",
  "txid": "string",
  "extraInfo": "string"
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorFields](#schemaerrorfields)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» type|any|false|none|none|
|» title|any|false|none|none|
|» status|any|false|none|none|
|» detail|any|false|none|none|
|» instance|any|false|none|none|

<h2 id="tocS_ErrorCumulativeFees">ErrorCumulativeFees</h2>
<!-- backwards compatibility -->
<a id="schemaerrorcumulativefees"></a>
<a id="schema_ErrorCumulativeFees"></a>
<a id="tocSerrorcumulativefees"></a>
<a id="tocserrorcumulativefees"></a>

```json
{
  "type": "https://bitcoin-sv.github.io/arc/#/errors?id=_473",
  "title": "Cumulative fee too low",
  "status": 473,
  "detail": "The cumulative fee is too low",
  "instance": "https://arc.taal.com/errors/123452",
  "txid": "string",
  "extraInfo": "string"
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ErrorFields](#schemaerrorfields)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» type|any|false|none|none|
|» title|any|false|none|none|
|» status|any|false|none|none|
|» detail|any|false|none|none|
|» instance|any|false|none|none|

<h2 id="tocS_Callback">Callback</h2>
<!-- backwards compatibility -->
<a id="schemacallback"></a>
<a id="schema_Callback"></a>
<a id="tocScallback"></a>
<a id="tocscallback"></a>

```json
{
  "timestamp": "string",
  "txid": "string",
  "txStatus": "string",
  "extraInfo": "string",
  "competingTxs": [
    "string"
  ],
  "merklePath": "string",
  "blockHash": "string",
  "blockHeight": 0
}

```

callback object

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|timestamp|string|true|none|none|
|txid|string|true|none|none|
|txStatus|string|true|none|none|
|extraInfo|string¦null|false|none|none|
|competingTxs|[string]|false|none|none|
|merklePath|string¦null|false|none|none|
|blockHash|string¦null|false|none|none|
|blockHeight|integer¦null|false|none|none|

<h2 id="tocS_BatchedCallbacks">BatchedCallbacks</h2>
<!-- backwards compatibility -->
<a id="schemabatchedcallbacks"></a>
<a id="schema_BatchedCallbacks"></a>
<a id="tocSbatchedcallbacks"></a>
<a id="tocsbatchedcallbacks"></a>

```json
{
  "value": {
    "count": 2,
    "callbacks": [
      {
        "timestamp": "2024-03-26T16:02:29.655390092Z",
        "competingTxs": [
          "505097ba93702491a8a8e5c195a3d2706baf9d43af5e8898aeace0e251f240d2"
        ],
        "txid": "3d7770a2c3bbf890fe69ad33faadd3efb470b60d05b071eaa86e6597d480e111",
        "txStatus": "DOUBLE_SPEND_ATTEMPTED"
      },
      {
        "timestamp": "2024-03-26T16:02:29.655390092Z",
        "txid": "48ccf56b16ec11ddd9cfafc4f28492fb7e989d58594a0acd150a1592570ccd13",
        "txStatus": "MINED",
        "merklePath": "fe12c70c000c020a008d1c719355d718dad0ccc...",
        "blockHash": "0000000000000000064cbaac5cedf71a5447771573ba585501952c023873817b",
        "blockHeight": 837394
      }
    ]
  }
}

```

batched callbacks object

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|count|integer|true|none|number of callbacks in response|
|callbacks|[[Callback](#schemacallback)]¦null|false|none|[callback object]|
