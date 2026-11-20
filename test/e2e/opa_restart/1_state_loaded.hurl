# policy from a ConfigMap
GET https://localhost:8443/v1/data/example/include/allow
Authorization: Bearer {{token}}

HTTP 200
[Asserts]
jsonpath "$.result" == true

# data from a ConfigMap
GET https://localhost:8443/v1/data/default/data-include
Authorization: Bearer {{token}}

HTTP 200
[Asserts]
jsonpath "$.result['include.json'].inKey" == "inValue"

# replicated resource
GET https://localhost:8443/v1/data/kubernetes/services/default/kubernetes
Authorization: Bearer {{token}}

HTTP 200
[Asserts]
jsonpath "$.result.metadata.name" == "kubernetes"
