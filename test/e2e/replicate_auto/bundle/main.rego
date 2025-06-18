package main

import rego.v1

main if {
    some ns, name
    data.kubernetes.services[ns][name].metadata.labels == "foo"
}