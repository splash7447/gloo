## glooctl create upstream azure

Create an Azure Upstream

### Synopsis

Azure Upstreams represent a set of Azure Functions for a Function App that can be routed to with Gloo. Azure Upstreams require a valid set of Azure Credentials to be provided. These should be uploaded to Gloo using `glooctl create secret azure`

```
glooctl create upstream azure [flags]
```

### Options

```
      --azure-app-name string                                       name of the Azure Functions app to associate with this upstream
      --azure-secret-name glooctl create secret azure --help        name of a secret containing Azure credentials created with glooctl. See glooctl create secret azure --help for help creating secrets (default "gloo-system")
      --azure-secret-namespace glooctl create secret azure --help   namespace where the Azure secret lives. See glooctl create secret azure --help for help creating secrets (default "gloo-system")
  -h, --help                                                        help for azure
      --name string                                                 name of the resource to read or write
  -n, --namespace string                                            namespace for reading or writing resources (default "gloo-system")
```

### Options inherited from parent commands

```
  -i, --interactive     use interactive mode
  -o, --output string   output format: (yaml, json, table)
```

### SEE ALSO

* [glooctl create upstream](glooctl_create_upstream.md)	 - Create an Upstream Interactively
