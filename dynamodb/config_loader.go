package dynamodb

import (
	"fmt"
	"github.com/mumoshu/division/api"
	"github.com/mumoshu/division/framework"
)

const databasePrefix = "div-"

const (
	crdName = "customresourcedefinition"
	crdKind = "CustomResourceDefinition"
)

func LoadConfigFromDynamoDB(table string, context *api.Config) (*api.Config, error) {
	db, err := newDefaultDynamoDBClient()
	if err != nil {
		return nil, err
	}
	if table != "" {
		return nil, fmt.Errorf(`not implemented error: table "%s" is specified, but it is unsupported`, table)
	}

	dynamicRDs, err := getCRDs(db, context)

	rdOfDynamicRDs := api.CustomResourceDefinition{
		Kind: crdKind,
		Metadata: api.Metadata{
			Name: crdName,
		},
		Spec: api.CustomResourceDefinitionSpec{
			Names: api.CustomResourceDefinitionNames{
				Kind: crdKind,
			},
		},
	}
	rds := []api.CustomResourceDefinition{
		rdOfDynamicRDs,
	}
	rds = append(rds, dynamicRDs...)
	return &api.Config{
		Metadata: api.Metadata{
			Name: context.Metadata.Name,
		},
		Spec: api.ConfigSpec{
			CustomResourceDefinitions: rds,
		},
	}, nil
}

func init() {
	framework.ConfigLoaders.Register("dynamodb", LoadConfigFromDynamoDB)
}
