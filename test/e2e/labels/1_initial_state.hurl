GET http://localhost:8080/v1/data
HTTP 200
[Asserts]
jsonpath "$.result.*" count == 0
