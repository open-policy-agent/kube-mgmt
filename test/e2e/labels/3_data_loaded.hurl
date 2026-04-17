GET http://localhost:8080/v1/data/default
HTTP 200
[Asserts]
jsonpath "$.result.*" count == 1
jsonpath "$.result.data-include" exists

GET http://localhost:8080/v1/data/default/data-include
HTTP 200
[Asserts]
jsonpath "$.result['include.json'].inKey" == "inValue"
