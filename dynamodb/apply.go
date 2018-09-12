package dynamodb

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/guregu/dynamo"
	"github.com/mumoshu/division/api"
	"github.com/mumoshu/division/framework"
	"os"
	"time"
)

type PutItemOption struct {
	TableName string `validate:"required"`
	Format    string
	File      string
}

func (p *dynamoResourceDB) ApplyFile(file string) error {
	return p.applyFile(file)
}

func (p *dynamoResourceDB) applyFile(file string) error {
	resource, err := framework.LoadResourceFromYamlFile(file)
	if err != nil {
		return err
	}
	return p.Apply(resource)
}

func (p *dynamoResourceDB) Apply(resource *api.Resource) error {
	resource.Metadata.CreationTimestamp = time.Now()
	resource.Metadata.UpdateTimestamp = time.Now()

	var resourceDef *api.CustomResourceDefinition
	kind := resource.Kind
	for _, r := range p.resourceDefs {
		if r.ResourceKind() == kind {
			resourceDef = &r
			break
		}
	}
	if resourceDef == nil {
		return fmt.Errorf("no resource definition found in %v: name=%s kind=%s", p.resourceDefs, resource.NameHashKey, kind)
	}
	var err error
	existing := api.Resource{}
	var getErr error
	{
		for {
			getErr = p.namespacedTable(resourceDef).Get(HashKeyName, resource.Metadata.Name).One(&existing)
			if getErr != nil {
				if getErr, ok := err.(awserr.Error); ok {
					switch getErr.Code() {
					case dynamodb.ErrCodeResourceNotFoundException:
						break
					case dynamodb.ErrCodeProvisionedThroughputExceededException, dynamodb.ErrCodeInternalServerError, dynamodb.ErrCodeLimitExceededException:
						fmt.Fprintf(os.Stderr, "retrying on error: %v\n", getErr)
						time.Sleep(3 * time.Second)
						continue
					default:
						return fmt.Errorf("[bug] get: unexpected error: %v", getErr)
					}

				}
			}
			break
		}
	}
	if getErr == nil {
		resource.Metadata.CreationTimestamp = existing.Metadata.CreationTimestamp
	}
	err = p.namespacedTable(resourceDef).Put(resource).Run()
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case dynamodb.ErrCodeResourceNotFoundException:
			if err := p.db.CreateTable(p.tableNameForResourceNamed(resourceDef.Metadata.Name), resource).Stream(dynamo.NewImageView).Run(); err != nil {
				return err
			}
			for {
				err = p.namespacedTable(resourceDef).Put(resource).Run()
				if err != nil {
					if aerr, ok := err.(awserr.Error); ok {
						switch aerr.Code() {
						case dynamodb.ErrCodeResourceNotFoundException, dynamodb.ErrCodeResourceInUseException:
							fmt.Fprintf(os.Stderr, "retrying on error: %v: table may be creating...\n", aerr.Error())
							time.Sleep(5 * time.Second)
							continue
						}
					}
					return fmt.Errorf("unexpected error while waiting for table creation: %v", err)
				}
				break
			}
		}
	}
	if err != nil {
		return fmt.Errorf("unexpected error: %v", err)
	}
	if getErr == nil {
		fmt.Printf("%s \"%s\" updated\n", resourceDef.Metadata.Name, resource.Metadata.Name)
	} else {
		fmt.Printf("%s \"%s\" created\n", resourceDef.Metadata.Name, resource.Metadata.Name)
	}
	return nil
}
