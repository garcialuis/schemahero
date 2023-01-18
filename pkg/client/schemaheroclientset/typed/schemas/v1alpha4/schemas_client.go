/*
Copyright 2021 The SchemaHero Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by client-gen. DO NOT EDIT.

package v1alpha4

import (
	v1alpha4 "github.com/schemahero/schemahero/pkg/apis/schemas/v1alpha4"
	"github.com/schemahero/schemahero/pkg/client/schemaheroclientset/scheme"
	rest "k8s.io/client-go/rest"
)

type SchemasV1alpha4Interface interface {
	RESTClient() rest.Interface
	DataTypesGetter
	MigrationsGetter
	TablesGetter
	ViewsGetter
}

// SchemasV1alpha4Client is used to interact with features provided by the schemas.schemahero.io group.
type SchemasV1alpha4Client struct {
	restClient rest.Interface
}

func (c *SchemasV1alpha4Client) DataTypes(namespace string) DataTypeInterface {
	return newDataTypes(c, namespace)
}

func (c *SchemasV1alpha4Client) Migrations(namespace string) MigrationInterface {
	return newMigrations(c, namespace)
}

func (c *SchemasV1alpha4Client) Tables(namespace string) TableInterface {
	return newTables(c, namespace)
}

func (c *SchemasV1alpha4Client) Views(namespace string) ViewInterface {
	return newViews(c, namespace)
}

// NewForConfig creates a new SchemasV1alpha4Client for the given config.
func NewForConfig(c *rest.Config) (*SchemasV1alpha4Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &SchemasV1alpha4Client{client}, nil
}

// NewForConfigOrDie creates a new SchemasV1alpha4Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *SchemasV1alpha4Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new SchemasV1alpha4Client for the given RESTClient.
func New(c rest.Interface) *SchemasV1alpha4Client {
	return &SchemasV1alpha4Client{c}
}

func setConfigDefaults(config *rest.Config) error {
	gv := v1alpha4.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *SchemasV1alpha4Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
