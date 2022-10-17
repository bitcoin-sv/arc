---
title: Merchant API v2.0.0
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

<h1 id="merchant-api">Merchant API v2.0.0</h1>

> Scroll down for code samples, example requests and responses. Select a language for code samples from the tabs above or the mobile navigation menu.

# Authentication

* API Key (Bearer)
    - Parameter Name: **Authorization**, in: header. Please enter JWT with Bearer needed to access MAPI into field. Authorization: Bearer JWT

* API Key (Api-Key)
    - Parameter Name: **Api-Key**, in: header. Please enter API key needed to access admin endpoints into field. Api-Key: My_API_Key

<h1 id="merchant-api-mapi">Mapi</h1>

## Get a fee quote.

> Code samples

```http
GET /mapi/v2/feeQuote HTTP/1.1

Accept: application/json

```

```javascript

const headers = {
  'Accept':'application/json',
  'Authorization':'API_KEY'
};

fetch('/mapi/v2/feeQuote',
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
URL obj = new URL("/mapi/v2/feeQuote");
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
        "Authorization": []string{"API_KEY"},
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("GET", "/mapi/v2/feeQuote", data)
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
  'Authorization' => 'API_KEY'
}

result = RestClient.get '/mapi/v2/feeQuote',
  params: {
  }, headers: headers

p JSON.parse(result)

```

```python
import requests
headers = {
  'Accept': 'application/json',
  'Authorization': 'API_KEY'
}

r = requests.get('/mapi/v2/feeQuote', headers = headers)

print(r.json())

```

```shell
# You can also use wget
curl -X GET /mapi/v2/feeQuote \
  -H 'Accept: application/json' \
  -H 'Authorization: API_KEY'

```

`GET /mapi/v2/feeQuote`

This endpoint returns the fees charged by a specific BSV miner.

> Example responses

> 200 Response

```json
{
  "apiVersion": "string",
  "timestamp": "2019-08-24T14:15:22Z",
  "minerId": "string",
  "expiryTime": "2019-08-24T14:15:22Z",
  "fees": [
    {
      "feeType": "string",
      "miningFee": {
        "satoshis": 0,
        "bytes": 0
      },
      "relayFee": {
        "satoshis": 0,
        "bytes": 0
      }
    }
  ]
}
```

<h3 id="get-a-fee-quote.-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Success|[FeeQuote](#schemafeequote)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|The requester is unauthorized.|None|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Error|[Error](#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|200|signature|string||ECDSA signature for response|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
Bearer, Api-Key
</aside>

## Get a policy quote.

> Code samples

```http
GET /mapi/v2/policyQuote HTTP/1.1

Accept: application/json

```

```javascript

const headers = {
  'Accept':'application/json',
  'Authorization':'API_KEY'
};

fetch('/mapi/v2/policyQuote',
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
URL obj = new URL("/mapi/v2/policyQuote");
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
        "Authorization": []string{"API_KEY"},
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("GET", "/mapi/v2/policyQuote", data)
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
  'Authorization' => 'API_KEY'
}

result = RestClient.get '/mapi/v2/policyQuote',
  params: {
  }, headers: headers

p JSON.parse(result)

```

```python
import requests
headers = {
  'Accept': 'application/json',
  'Authorization': 'API_KEY'
}

r = requests.get('/mapi/v2/policyQuote', headers = headers)

print(r.json())

```

```shell
# You can also use wget
curl -X GET /mapi/v2/policyQuote \
  -H 'Accept: application/json' \
  -H 'Authorization: API_KEY'

```

`GET /mapi/v2/policyQuote`

This endpoint returns the fees charged by a specific BSV miner and set policies.

> Example responses

> 200 Response

```json
{
  "apiVersion": "string",
  "timestamp": "2019-08-24T14:15:22Z",
  "minerId": "string",
  "expiryTime": "2019-08-24T14:15:22Z",
  "fees": [
    {
      "feeType": "string",
      "miningFee": {
        "satoshis": 0,
        "bytes": 0
      },
      "relayFee": {
        "satoshis": 0,
        "bytes": 0
      }
    }
  ],
  "callbacks": [
    {
      "url": "string",
      "token": "string"
    }
  ],
  "policies": {
    "property1": null,
    "property2": null
  }
}
```

<h3 id="get-a-policy-quote.-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Success|[PolicyQuote](#schemapolicyquote)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|The requester is unauthorized.|None|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Error|[Error](#schemaerror)|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
Bearer, Api-Key
</aside>

## Query transaction status.

> Code samples

```http
GET /mapi/v2/tx/{id} HTTP/1.1

Accept: application/json

```

```javascript

const headers = {
  'Accept':'application/json',
  'Authorization':'API_KEY'
};

fetch('/mapi/v2/tx/{id}',
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
URL obj = new URL("/mapi/v2/tx/{id}");
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
        "Authorization": []string{"API_KEY"},
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("GET", "/mapi/v2/tx/{id}", data)
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
  'Authorization' => 'API_KEY'
}

result = RestClient.get '/mapi/v2/tx/{id}',
  params: {
  }, headers: headers

p JSON.parse(result)

```

```python
import requests
headers = {
  'Accept': 'application/json',
  'Authorization': 'API_KEY'
}

r = requests.get('/mapi/v2/tx/{id}', headers = headers)

print(r.json())

```

```shell
# You can also use wget
curl -X GET /mapi/v2/tx/{id} \
  -H 'Accept: application/json' \
  -H 'Authorization: API_KEY'

```

`GET /mapi/v2/tx/{id}`

This endpoint is used to check the current status of a previously submitted transaction.

<h3 id="query-transaction-status.-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|The transaction ID (32 byte hash) hex string|

> Example responses

> 200 Response

```json
{
  "apiVersion": "string",
  "timestamp": "2019-08-24T14:15:22Z",
  "minerId": "string",
  "txid": "string",
  "statusCode": 0,
  "status": "string",
  "blockHash": "string",
  "blockHeight": 0,
  "confirmations": 0,
  "txSecondMempoolExpiry": 0
}
```

<h3 id="query-transaction-status.-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Success|[TransactionStatus](#schematransactionstatus)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|The requester is unauthorized.|None|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Error|[Error](#schemaerror)|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
Bearer, Api-Key
</aside>

## Submit a transaction.

> Code samples

```http
POST /mapi/v2/tx HTTP/1.1

Content-Type: application/json
Accept: application/json

```

```javascript
const inputBody = '{
  "rawTx": "string",
  "callbackUrl": "string",
  "callbackToken": "string",
  "merkleProof": true
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json',
  'Authorization':'API_KEY'
};

fetch('/mapi/v2/tx',
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
URL obj = new URL("/mapi/v2/tx");
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
        "Content-Type": []string{"application/json"},
        "Accept": []string{"application/json"},
        "Authorization": []string{"API_KEY"},
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("POST", "/mapi/v2/tx", data)
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
  'Content-Type' => 'application/json',
  'Accept' => 'application/json',
  'Authorization' => 'API_KEY'
}

result = RestClient.post '/mapi/v2/tx',
  params: {
  }, headers: headers

p JSON.parse(result)

```

```python
import requests
headers = {
  'Content-Type': 'application/json',
  'Accept': 'application/json',
  'Authorization': 'API_KEY'
}

r = requests.post('/mapi/v2/tx', headers = headers)

print(r.json())

```

```shell
# You can also use wget
curl -X POST /mapi/v2/tx \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: API_KEY'

```

`POST /mapi/v2/tx`

This endpoint is used to send a raw transaction to a miner for inclusion in the next block that the miner creates.

> Body parameter

```json
{
  "rawTx": "string",
  "callbackUrl": "string",
  "callbackToken": "string",
  "merkleProof": true
}
```

<h3 id="submit-a-transaction.-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[SubmitTransaction](#schemasubmittransaction)|false|none|

> Example responses

> 200 Response

```json
{
  "apiVersion": "string",
  "timestamp": "2019-08-24T14:15:22Z",
  "minerId": "string",
  "txid": "string",
  "statusCode": 0,
  "status": "string",
  "currentHighestBlockHash": "string",
  "currentHighestBlockHeight": 0,
  "txSecondMempoolExpiry": 0,
  "conflictedWith": [
    {
      "txid": "string",
      "size": 0,
      "hex": "string"
    }
  ]
}
```

<h3 id="submit-a-transaction.-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Success|[SubmitTransactionResponse](#schemasubmittransactionresponse)|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
Bearer, Api-Key
</aside>

## Submit multiple transactions.

> Code samples

```http
POST /mapi/v2/txs HTTP/1.1

Content-Type: application/json
Accept: application/json

```

```javascript
const inputBody = '[
  {
    "rawTx": "string",
    "callbackUrl": "string",
    "callbackToken": "string",
    "merkleProof": true
  }
]';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json',
  'Authorization':'API_KEY'
};

fetch('/mapi/v2/txs',
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
URL obj = new URL("/mapi/v2/txs");
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
        "Content-Type": []string{"application/json"},
        "Accept": []string{"application/json"},
        "Authorization": []string{"API_KEY"},
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("POST", "/mapi/v2/txs", data)
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
  'Content-Type' => 'application/json',
  'Accept' => 'application/json',
  'Authorization' => 'API_KEY'
}

result = RestClient.post '/mapi/v2/txs',
  params: {
  }, headers: headers

p JSON.parse(result)

```

```python
import requests
headers = {
  'Content-Type': 'application/json',
  'Accept': 'application/json',
  'Authorization': 'API_KEY'
}

r = requests.post('/mapi/v2/txs', headers = headers)

print(r.json())

```

```shell
# You can also use wget
curl -X POST /mapi/v2/txs \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: API_KEY'

```

`POST /mapi/v2/txs`

This endpoint is used to send multiple raw transactions to a miner for inclusion in the next block that the miner creates.

> Body parameter

```json
[
  {
    "rawTx": "string",
    "callbackUrl": "string",
    "callbackToken": "string",
    "merkleProof": true
  }
]
```

<h3 id="submit-multiple-transactions.-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|defaultCallbackUrl|query|string|false|Default double spend and merkle proof notification callback endpoint.|
|defaultCallbackToken|query|string|false|Default access token for notification callback endpoint.|
|body|body|[SubmitTransaction](#schemasubmittransaction)|false|none|

> Example responses

> 200 Response

```json
{
  "apiVersion": "string",
  "timestamp": "2019-08-24T14:15:22Z",
  "minerId": "string",
  "txid": "string",
  "statusCode": 0,
  "status": "string",
  "currentHighestBlockHash": "string",
  "currentHighestBlockHeight": 0,
  "txSecondMempoolExpiry": 0,
  "conflictedWith": [
    {
      "txid": "string",
      "size": 0,
      "hex": "string"
    }
  ]
}
```

<h3 id="submit-multiple-transactions.-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Success|[SubmitTransactionResponse](#schemasubmittransactionresponse)|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
Bearer, Api-Key
</aside>

# Schemas

<h2 id="tocS_BasicResponse">BasicResponse</h2>
<!-- backwards compatibility -->
<a id="schemabasicresponse"></a>
<a id="schema_BasicResponse"></a>
<a id="tocSbasicresponse"></a>
<a id="tocsbasicresponse"></a>

```json
{
  "apiVersion": "string",
  "timestamp": "2019-08-24T14:15:22Z",
  "minerId": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|false|none|none|
|timestamp|string(date-time)|false|none|none|
|minerId|string¦null|false|none|none|

<h2 id="tocS_Callback">Callback</h2>
<!-- backwards compatibility -->
<a id="schemacallback"></a>
<a id="schema_Callback"></a>
<a id="tocScallback"></a>
<a id="tocscallback"></a>

```json
{
  "url": "string",
  "token": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|url|string¦null|false|none|none|
|token|string¦null|false|none|none|

<h2 id="tocS_FeeAmount">FeeAmount</h2>
<!-- backwards compatibility -->
<a id="schemafeeamount"></a>
<a id="schema_FeeAmount"></a>
<a id="tocSfeeamount"></a>
<a id="tocsfeeamount"></a>

```json
{
  "satoshis": 0,
  "bytes": 0
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|satoshis|integer(uint64)|false|none|none|
|bytes|integer(uint64)|false|none|none|

<h2 id="tocS_FeeQuote">FeeQuote</h2>
<!-- backwards compatibility -->
<a id="schemafeequote"></a>
<a id="schema_FeeQuote"></a>
<a id="tocSfeequote"></a>
<a id="tocsfeequote"></a>

```json
{
  "apiVersion": "string",
  "timestamp": "2019-08-24T14:15:22Z",
  "minerId": "string",
  "expiryTime": "2019-08-24T14:15:22Z",
  "fees": [
    {
      "feeType": "string",
      "miningFee": {
        "satoshis": 0,
        "bytes": 0
      },
      "relayFee": {
        "satoshis": 0,
        "bytes": 0
      }
    }
  ]
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[BasicResponse](#schemabasicresponse)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» expiryTime|string(date-time)|false|none|none|
|» fees|[[Fee](#schemafee)]¦null|false|none|none|

<h2 id="tocS_Fee">Fee</h2>
<!-- backwards compatibility -->
<a id="schemafee"></a>
<a id="schema_Fee"></a>
<a id="tocSfee"></a>
<a id="tocsfee"></a>

```json
{
  "feeType": "string",
  "miningFee": {
    "satoshis": 0,
    "bytes": 0
  },
  "relayFee": {
    "satoshis": 0,
    "bytes": 0
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|feeType|string¦null|false|none|none|
|miningFee|[FeeAmount](#schemafeeamount)|false|none|none|
|relayFee|[FeeAmount](#schemafeeamount)|false|none|none|

<h2 id="tocS_PolicyQuote">PolicyQuote</h2>
<!-- backwards compatibility -->
<a id="schemapolicyquote"></a>
<a id="schema_PolicyQuote"></a>
<a id="tocSpolicyquote"></a>
<a id="tocspolicyquote"></a>

```json
{
  "apiVersion": "string",
  "timestamp": "2019-08-24T14:15:22Z",
  "minerId": "string",
  "expiryTime": "2019-08-24T14:15:22Z",
  "fees": [
    {
      "feeType": "string",
      "miningFee": {
        "satoshis": 0,
        "bytes": 0
      },
      "relayFee": {
        "satoshis": 0,
        "bytes": 0
      }
    }
  ],
  "callbacks": [
    {
      "url": "string",
      "token": "string"
    }
  ],
  "policies": {
    "property1": null,
    "property2": null
  }
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[BasicResponse](#schemabasicresponse)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[FeeQuote](#schemafeequote)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» callbacks|[[Callback](#schemacallback)]¦null|false|none|none|
|» policies|object¦null|false|none|none|
|»» **additionalProperties**|any|false|none|none|

<h2 id="tocS_TransactionStatus">TransactionStatus</h2>
<!-- backwards compatibility -->
<a id="schematransactionstatus"></a>
<a id="schema_TransactionStatus"></a>
<a id="tocStransactionstatus"></a>
<a id="tocstransactionstatus"></a>

```json
{
  "apiVersion": "string",
  "timestamp": "2019-08-24T14:15:22Z",
  "minerId": "string",
  "txid": "string",
  "statusCode": 0,
  "status": "string",
  "blockHash": "string",
  "blockHeight": 0,
  "confirmations": 0,
  "txSecondMempoolExpiry": 0
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[BasicResponse](#schemabasicresponse)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» txid|string|false|none|none|
|» statusCode|number|false|none|none|
|» status|string|false|none|none|
|» blockHash|string¦null|false|none|none|
|» blockHeight|integer(int64)¦null|false|none|none|
|» confirmations|integer(int64)¦null|false|none|none|
|» minerId|string¦null|false|none|none|
|» txSecondMempoolExpiry|integer(int32)¦null|false|none|none|

<h2 id="tocS_SubmitTransactionConflicted">SubmitTransactionConflicted</h2>
<!-- backwards compatibility -->
<a id="schemasubmittransactionconflicted"></a>
<a id="schema_SubmitTransactionConflicted"></a>
<a id="tocSsubmittransactionconflicted"></a>
<a id="tocssubmittransactionconflicted"></a>

```json
{
  "txid": "string",
  "size": 0,
  "hex": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|txid|string¦null|false|none|none|
|size|integer(int64)|false|none|none|
|hex|string¦null|false|none|none|

<h2 id="tocS_SubmitTransactionResponse">SubmitTransactionResponse</h2>
<!-- backwards compatibility -->
<a id="schemasubmittransactionresponse"></a>
<a id="schema_SubmitTransactionResponse"></a>
<a id="tocSsubmittransactionresponse"></a>
<a id="tocssubmittransactionresponse"></a>

```json
{
  "apiVersion": "string",
  "timestamp": "2019-08-24T14:15:22Z",
  "minerId": "string",
  "txid": "string",
  "statusCode": 0,
  "status": "string",
  "currentHighestBlockHash": "string",
  "currentHighestBlockHeight": 0,
  "txSecondMempoolExpiry": 0,
  "conflictedWith": [
    {
      "txid": "string",
      "size": 0,
      "hex": "string"
    }
  ]
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[BasicResponse](#schemabasicresponse)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» txid|string|false|none|none|
|» statusCode|number|false|none|none|
|» status|string|false|none|none|
|» currentHighestBlockHash|string¦null|false|none|none|
|» currentHighestBlockHeight|integer(int64)¦null|false|none|none|
|» txSecondMempoolExpiry|integer(int64)¦null|false|none|none|
|» conflictedWith|[[SubmitTransactionConflicted](#schemasubmittransactionconflicted)]¦null|false|none|none|

<h2 id="tocS_SubmitTransaction">SubmitTransaction</h2>
<!-- backwards compatibility -->
<a id="schemasubmittransaction"></a>
<a id="schema_SubmitTransaction"></a>
<a id="tocSsubmittransaction"></a>
<a id="tocssubmittransaction"></a>

```json
{
  "rawTx": "string",
  "callbackUrl": "string",
  "callbackToken": "string",
  "merkleProof": true
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|rawTx|string|false|none|none|
|callbackUrl|string¦null|false|none|none|
|callbackToken|string¦null|false|none|none|
|merkleProof|boolean¦null|false|none|none|

<h2 id="tocS_Error">Error</h2>
<!-- backwards compatibility -->
<a id="schemaerror"></a>
<a id="schema_Error"></a>
<a id="tocSerror"></a>
<a id="tocserror"></a>

```json
{
  "code": 0,
  "message": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|code|number|true|none|none|
|message|string|true|none|none|

