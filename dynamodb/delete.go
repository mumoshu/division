package dynamodb

import (
	"fmt"
	"github.com/guregu/dynamo"
	"github.com/mumoshu/division/api"
)

func (p *dynamoResourceDB) Delete(resource string, name string) error {
	err := p.tableForResourceNamed(resource).Delete(HashKeyName, partitionKey(name)).OldValue(&api.Resource{})
	if err != nil {
		// Small trick to make the error message a bit nicer
		//
		// BEFORE:
		//   Error: dynamo: no item found
		// AFTER:
		//   Error: cluster "foo" not found
		if err == dynamo.ErrNotFound {
			return fmt.Errorf(`%s "%s" not found`, resource, name)
		}
	} else {
		fmt.Printf("%s \"%s\" deleted\n", resource, name)
	}
	return err
}
