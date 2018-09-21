Division
=======

A single-binary control-plane for your Kubernetes clusters

Composed of several brigades, division delegates common tasks seen while operating distributed microservices on Kubenretes.

## Introduction

`Division` is a kind of control-plane for an event-based scripting platform for Kubernetes [Brigade](https://github.com/Azure/brigade).

It is capable of distributing, scheduling, and logging any Kubernetes pods, runnable as Brigade scripts, across multiple Kubernetes clusters.

Run it from your local machine or continuous integration system, as part of
your GitOps or continous delivery pipeline, blue-green Kubernetes cluster switch, disaster recovery, and so on.

## Model use-case: DIY GitOps Pipeline

For demonstration purpose, here's how you'd implement a GitOps pipeline for your monorepo with `Division`.

Firstly, install division's brigade gateway into your Kubernetes cluster:

```
$ ./init mumoshu/uuid-generator app1
```

And run the gateway locally, or remotely via the helm chart:

```
$ div gateway --cluster mycluster1
```

Then, run the `div deploy` command to deploy your project onto the specified Kubernetes cluster:

```
$ div deploy \
  --project mumoshu/microservices \
  --app mumoshu/microservices/myapp1 \
  --ref $COMMIT_ID 
  --cluster mycluster1
```

As soon as `div deloy` is run, the actual deployment using [helmfile](https://github.com/roboll/helmfile) is executed,
and its logs are streamed from the gateway into `div deploy`.

What's important is that this happens without direct access to Kubernets API.

So even you are an user of publicly hosted CI/CD service like CircleCI or Travis CI, you can use `div` to deploy your Kubernetes applications,
without exposing the Kubernetes API server to Internet.

Repeat the `div gateway` for each cluster and omit `--cluster` flags from `div deploy`
to make it multi-cluster aware.

## Design

`Division` has three components - `store`, `div`, and `gateway`.

### store

`store` is the core of `Division`. It is a a general-purpose, schemaless database
that is very similar to Kubernetes's custom resources.
This means, you can build something like [Kubernetes Operators](https://coreos.com/operators/) without Kubernetes.

`store` is currently implemented by `DynamoDB`, so that `division` acts as a proxy to create/read/update/delete.
DynamoDB tables and items. Although it is the only supported backend as of today, alternative implementation can be
easily plugged-in by writing small amount of golang code.

One of interesting featuers of `store` is that it provides a log stream per resource.
So let's say you have a `deployment` resource, you can write and read log messages associated to the `deployment` resource.
Use it for `installation` + `installation logs`, `deployment` + `deployment logs`, `job` + `job logs`, and so on.

### div

`div` is the command-line interface to `Division`.
It can be used as a kubectl-like interface to manage your custom resources persisted in the datastore.

### gateway

`gateway` a.k.a `div gateway` is the sub-command of `div`, that acts as a [brigade gateway](https://github.com/Azure/brigade/blob/master/docs/topics/gateways.md).
It receives any newly created and/or updated resources in `store`, translating to brigade events,
that ends up triggering brigade scripts that orchestrates Kubernetes pods to achieve useful tasks.

## Use cases

### Centralize shared configuration of your CI pipelines

`div` is intended to compliment both Pipeline-based CI systems and your workflows, so that you can use
`div` as a source-of-truth across all your automations.

### Kubernetes Cluster Discovery

For example, you may use `div` to discover your clusters from the CI/CD pipeline, without maintaining
a list of known clusters within your application repositories.

### Implementing Event Hub With Minimum Moving Parts

In near future, `div wait` allows you to build an event hub for your system.

For example, a `wait until human approval` workflow that is useful in your CI/CD pipeline can be implemented simply like:

The requester would run `myjob1`:

```console
echo waiting for approval of job: myjob1

div wait approval myjob1approval --until jsonql="status.phase = 'Approved'"

echo myjob1 has been approved. continuing the process...
```

Whereas the approver approves the job:

```console
$ div get myjob1approval -o json > myjob1approval.unapproved.json
$ ./json-set "status.phase=Approved" myjob1approval.unapproved.json > myjob1approval.approved.json 
$ div apply -f myjob1approval.approved.json 
``` 

## Installation

```
$ go get github.com/mumoshu/div
```

## Getting started

1. Create an IAM user for `division` whose policy contains:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "AllAPIActionsOnUserSpecificTable",
            "Effect": "Allow",
            "Action": [
                "dynamodb:*"
            ],
            "Resource": "arn:aws:dynamodb:YOUR_REGION:YOUR_AWS_ACCOUNT:table/crdb*"
        },
        {
            "Sid": "AdditionalPrivileges",
            "Effect": "Allow",
            "Action": [
                "dynamodb:ListTables",
                "dynamodb:DescribeTable",
                "cloudwatch:*"
            ],
            "Resource": "*"
        }
    ]
}
```

2. Create `div.yaml`:

```yaml
metadata:
  name: dynamic
spec:
  source: "dynamodb://"
```

Now, you are ready to CRUD your resources by running `div`.

3. Provide a proper AWS credentials to `div` via envvars(`AWS_PROFILE` is supported, too) or an instance profile.

4. Create resource definitions for resources used by `division`.
 
This results in creating a few DynamoDB tables, and adding corresponding table items to them.
 
To do so, run `div apply -f FILE` command for each resource defitinion:

```console
for resource in application cluster deployment install release project; do
  div apply -f ${resource}.crd.yaml
done
```

5. Install `brigade` and `div gateway` into your Kubernets cluster by  running:

```
$ helm repo add brigade https://azure.github.io/brigade
$ helm install brigade/brigade --set rbac.enabled=true

$ git clone https://github.com/mumoshu/division.git
$ cd division
$ git checkout $(git tag -l | tail -n 1)
$ helm install ./charts/division --set rbac.enabled=true
```

6. Deploy the example application onto your cluster via `division`:

```
$ ./init mumoshu/uuid-generator app1

$ div deploy \
  --project mumoshu/microservices \
  --app mumoshu/microservices/myapp1 \
  --ref $COMMIT_ID
  --cluster mycluster1
```

## Usage

### Get

```
Displays one or more resources

Examples:
  # list all myresources
  div get myresources 

  # list a single myresource with specified name 
  div get myresource foo

  # list a myresource identified by name in JSON output format
  div get myresource foo -o json

  # list myresources whose labels match the specified selector
  div get myresources -l foo=bar 
```

### Apply

```
Apply a configuration to a resource by filename. The resource name must be specified in the file.
This resource will be created if it doesn't exist yet.

Examples:
  div apply [-f|--file] <FilePath>
```

### Delete

```
Delete resources by resources and names.

Examples:
  # Delete a myresource with specified name
  div delete myresource foo
```

## Configuration

Provide `div` your resource definitions via either `static` or `dynamic`(recommended) config.

### Static configuration

In static configuration, you provide one or more resource definitions in `div.yaml`:

```yaml
metadata:
  name: static
spec:
  customResourceDefinitions:
  - kind: CustomResourceDefinition
    metadata:
      name: cluster
    spec:
      names:
        kind: Cluster
```

Now you can CRUD the `cluster` resources by running `div` commands:

More concretely:

- `div apply -f yourcluster.yaml` to create a `cluster` resource. See `example/foo.cluster.yaml` for details on the yaml file.
- `div [get|delete] cluster foo` to get or delete a `cluster` named `foo`, respectively.

### Dynamic configuration(recommended)

In `dynamic` configuration, you just tell `div` to read resource definitions from the specified source.

An example `div.yaml` that loads resource definitions from a DynamoDB table named `div-dynamic-customresourcedefinitions` would look like:

```yaml
metadata:
  name: dynamic
spec:
  source: "dynamodb://"
```

Next, create your resource definition on DynamoDB:

```console
$ div apply -f example/cluster.crd.yaml
```

Assuming the `cluster.crd.yaml` looked like:

```yaml
kind: CustomResourceDefinition
metadata:
  name: cluster
spec:
  names:
    kind: Cluster
```

You can CRUD the `cluster` resources by running `div` commands:

More concretely:

- `div apply -f yourcluster.yaml` to create a `cluster` resource. See `example/foo.cluster.yaml` for details on the yaml file.
- `div [get|delete] cluster foo` to get or delete a `cluster` named `foo`, respectively.

## Roadmap

### List-Watch

- [x] `div get myresource --watch` to stream resource changes, including creations, updates, and deletions.

### Wait for condition

- [x] `div wait myresource foo "status.phase ~= 'Done.*'"` to wait until `myresource` named `foo` matches the [jsonql](https://github.com/elgs/jsonql) expression.
- [x] `div wait myresource foo ... --timeout 10s` adds timeout to the above

### Resource Logs

- [x] `div logs write myresource foo -f logs.txt` to write logs. logs can be streamed from another clients.
- [x] `div logs read myresource foo -f` to stream logs associated to the resource

### Usability Improvements

- [x] `div wait myresource foo --logs "status.phase = 'Done'"` to wait until `myresource` named `foo` matches the [jsonql](https://github.com/elgs/jsonql) expression, while streaming all the logs associated to the resource until the end
- [ ] `div wait --file foo.myresource.yaml --apply --logs --until jsonql="status.phase = 'Done'"` to create `myresource` named `foo` and wait until it comes to match the [jsonql](https://github.com/elgs/jsonql) expression, while streaming all the logs associated to the resource until the end
- [ ] `div gen iampolicy [readonly|writeonly|readwrite] myresource` to generate cloud IAM policy like AWS IAM policy to ease setting up least privilege for your developers
- [ ] `div template -f myresoruce.yaml.tpl --pipe-to "div apply -f -"` to consume ndjson input to apply execute the specified command with the input generated from the template
- [ ] `div init --source dynamodb` to generate `div.yaml`

## Contributing

Contributions to add another datastores from various clouds and OSSes are more than welcome!
See the `api` for the API which your additional datastore should support, whereas the `framework` pkg is to help implementing it.
Also, the `dynamodb` pkg is the example implementation of the `api`. You can even start by copying the whole `dynamodb` pkg into your own datastore pkg. 

Please feel free to ask @mumoshu if you had technical difficulty to do so. I'm here to help!

## Acknowledgement

This project is highly inspired by a DynamoDB CLI named [watarukura/gody](https://github.com/watarukura/gody).
A lot of thanks to @watarukura for sharing the awesome project!
