package database

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	schemasv1alpha4 "github.com/schemahero/schemahero/pkg/apis/schemas/v1alpha4"
	"github.com/schemahero/schemahero/pkg/database/cassandra"
	"github.com/schemahero/schemahero/pkg/database/mysql"
	"github.com/schemahero/schemahero/pkg/database/postgres"
	"github.com/schemahero/schemahero/pkg/database/rqlite"
	"github.com/schemahero/schemahero/pkg/database/sqlite"
	"github.com/schemahero/schemahero/pkg/database/timescaledb"
	"github.com/schemahero/schemahero/pkg/logger"
	"github.com/schemahero/schemahero/pkg/trace"
	"go.opentelemetry.io/otel"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

type Database struct {
	InputDir       string
	OutputDir      string
	Driver         string
	URI            string
	Hosts          []string
	Username       string
	Password       string
	Keyspace       string
	DeploySeedData bool
}

func (d *Database) CreateFixturesSync(ctx context.Context) error {
	var span oteltrace.Span
	ctx, span = otel.Tracer(trace.TraceName).Start(ctx, "CreateFixturesSync")
	defer span.End()

	logger.Info("generating fixtures",
		zap.String("input-dir", d.InputDir))

	statements := []string{}
	handleFile := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		fileData, err := ioutil.ReadFile(filepath.Join(d.InputDir, info.Name()))
		if err != nil {
			return err
		}

		var spec *schemasv1alpha4.TableSpec

		parsedK8sObject := schemasv1alpha4.Table{}
		if err := yaml.Unmarshal(fileData, &parsedK8sObject); err == nil {
			if parsedK8sObject.Spec.Schema != nil {
				spec = &parsedK8sObject.Spec
			}
		}

		if spec == nil {
			plainSpec := schemasv1alpha4.TableSpec{}
			if err := yaml.Unmarshal(fileData, &plainSpec); err != nil {
				return err
			}

			spec = &plainSpec
		}

		if spec.Schema == nil {
			return nil
		}

		if d.Driver == "postgres" {
			if spec.Schema.Postgres == nil {
				return nil
			}

			createStatements, err := postgres.CreateTableStatements(spec.Name, spec.Schema.Postgres)
			if err != nil {
				return err
			}

			statements = append(statements, createStatements...)
		} else if d.Driver == "mysql" {
			if spec.Schema.Mysql == nil {
				return nil
			}

			createStatements, err := mysql.CreateTableStatements(ctx, spec.Name, spec.Schema.Mysql)
			if err != nil {
				return err
			}

			statements = append(statements, createStatements...)
		} else if d.Driver == "cockroachdb" {
			if spec.Schema.CockroachDB == nil {
				return nil
			}

			createStatements, err := postgres.CreateTableStatements(spec.Name, spec.Schema.CockroachDB)
			if err != nil {
				return err
			}

			statements = append(statements, createStatements...)
		} else if d.Driver == "sqlite" {
			if spec.Schema.SQLite == nil {
				return nil
			}

			createStatements, err := sqlite.CreateTableStatements(spec.Name, spec.Schema.SQLite)
			if err != nil {
				return err
			}

			statements = append(statements, createStatements...)
		} else if d.Driver == "rqlite" {
			if spec.Schema.RQLite == nil {
				return nil
			}

			createStatements, err := rqlite.CreateTableStatements(spec.Name, spec.Schema.RQLite)
			if err != nil {
				return err
			}

			statements = append(statements, createStatements...)
		} else if d.Driver == "timescaledb" {
			if spec.Schema.TimescaleDB == nil {
				return nil
			}

			createStatements, err := timescaledb.CreateTableStatements(spec.Name, spec.Schema.TimescaleDB)
			if err != nil {
				return err
			}

			statements = append(statements, createStatements...)
		} else if d.Driver == "cassandra" {
			return errors.New("not implemented")
		}

		return nil
	}

	err := filepath.Walk(d.InputDir, handleFile)
	if err != nil {
		return err
	}

	output := strings.Join(statements, ";\n")
	output = fmt.Sprintf("/* Auto generated file. Do not edit by hand. This file was generated by SchemaHero. */\n\n %s;\n\n", output)

	if _, err := os.Stat(d.OutputDir); os.IsNotExist(err) {
		os.MkdirAll(d.OutputDir, 0750)
	}

	err = ioutil.WriteFile(filepath.Join(d.OutputDir, "fixtures.sql"), []byte(output), 0600)
	if err != nil {
		return err
	}

	return nil
}

func (d *Database) PlanSyncFromFile(ctx context.Context, filename string, specType string) ([]string, error) {
	specContents, err := ioutil.ReadFile(filepath.Clean(filename))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file")
	}

	if specType == "table" {
		return d.planTableSync(ctx, specContents)
	} else if specType == "type" {
		return d.planTypeSync(ctx, specContents)
	}

	return nil, errors.New("unknown spec type")
}

func (d *Database) planTableSync(ctx context.Context, specContents []byte) ([]string, error) {
	var spec *schemasv1alpha4.TableSpec
	parsedK8sObject := schemasv1alpha4.Table{}
	if err := yaml.Unmarshal(specContents, &parsedK8sObject); err == nil {
		if parsedK8sObject.Spec.Schema != nil {
			spec = &parsedK8sObject.Spec
		}
	}

	if spec == nil {
		plainSpec := schemasv1alpha4.TableSpec{}
		if err := yaml.Unmarshal(specContents, &plainSpec); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal spec")
		}

		spec = &plainSpec
	}

	return d.PlanSyncTableSpec(ctx, spec)
}

func (d *Database) PlanSyncTableSpec(ctx context.Context, spec *schemasv1alpha4.TableSpec) ([]string, error) {
	var span oteltrace.Span
	ctx, span = otel.Tracer(trace.TraceName).Start(ctx, "PlanSyncTableSpec")
	defer span.End()

	if spec.Schema == nil {
		return []string{}, nil
	}

	var seedData *schemasv1alpha4.SeedData
	if d.DeploySeedData {
		seedData = spec.SeedData
	}

	if d.Driver == "postgres" {
		return postgres.PlanPostgresTable(ctx, d.URI, spec.Name, spec.Schema.Postgres, seedData)
	} else if d.Driver == "mysql" {
		return mysql.PlanMysqlTable(ctx, d.URI, spec.Name, spec.Schema.Mysql, seedData)
	} else if d.Driver == "cockroachdb" {
		return postgres.PlanPostgresTable(ctx, d.URI, spec.Name, spec.Schema.CockroachDB, seedData)
	} else if d.Driver == "cassandra" {
		return cassandra.PlanCassandraTable(ctx, d.Hosts, d.Username, d.Password, d.Keyspace, spec.Name, spec.Schema.Cassandra, seedData)
	} else if d.Driver == "sqlite" {
		return sqlite.PlanSqliteTable(ctx, d.URI, spec.Name, spec.Schema.SQLite, seedData)
	} else if d.Driver == "rqlite" {
		return rqlite.PlanRqliteTable(ctx, d.URI, spec.Name, spec.Schema.RQLite, seedData)
	} else if d.Driver == "timescaledb" {
		return timescaledb.PlanTimescaleDBTable(ctx, d.URI, spec.Name, spec.Schema.TimescaleDB, seedData)
	}

	return nil, errors.Errorf("unknown database driver: %q", d.Driver)
}

func (d *Database) PlanSyncSeedData(ctx context.Context, spec *schemasv1alpha4.TableSpec) ([]string, error) {
	var span oteltrace.Span
	ctx, span = otel.Tracer(trace.TraceName).Start(ctx, "PlanSyncSeedData")
	defer span.End()

	if spec.SeedData == nil {
		return []string{}, nil
	}

	if d.Driver == "postgres" {
		return postgres.SeedDataStatements(spec.Name, spec.Schema.Postgres, spec.SeedData)
	} else if d.Driver == "mysql" {
		return mysql.SeedDataStatements(ctx, spec.Name, spec.SeedData)
	} else if d.Driver == "cockroachdb" {
		return postgres.SeedDataStatements(spec.Name, spec.Schema.Postgres, spec.SeedData)
	} else if d.Driver == "cassandra" {
		return nil, errors.New("cassandra seed data is not implemented")
	} else if d.Driver == "sqlite" {
		return sqlite.SeedDataStatements(spec.Name, spec.SeedData)
	} else if d.Driver == "rqlite" {
		return rqlite.SeedDataStatements(spec.Name, spec.SeedData)
	} else if d.Driver == "timescaledb" {
		return timescaledb.SeedDataStatements(spec.Name, spec.Schema.TimescaleDB, spec.SeedData)
	}

	return nil, errors.Errorf("unknown database driver: %q", d.Driver)
}

func (d *Database) planTypeSync(ctx context.Context, specContents []byte) ([]string, error) {
	var spec *schemasv1alpha4.DataTypeSpec
	parsedK8sObject := schemasv1alpha4.DataType{}
	if err := yaml.Unmarshal(specContents, &parsedK8sObject); err == nil {
		if parsedK8sObject.Spec.Schema != nil {
			spec = &parsedK8sObject.Spec
		}
	}

	if spec == nil {
		plainSpec := schemasv1alpha4.DataTypeSpec{}
		if err := yaml.Unmarshal(specContents, &plainSpec); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal spec")
		}

		spec = &plainSpec
	}

	return d.PlanSyncTypeSpec(spec)
}

func (d *Database) PlanSyncTypeSpec(spec *schemasv1alpha4.DataTypeSpec) ([]string, error) {
	if spec.Schema == nil {
		return []string{}, nil
	}

	if d.Driver == "cassandra" {
		return cassandra.PlanCassandraType(d.Hosts, d.Username, d.Password, d.Keyspace, spec.Name, spec.Schema.Cassandra)
	}

	return nil, errors.Errorf("planning types is not supported for driver %q", d.Driver)
}

func (d *Database) ApplySync(ctx context.Context, statements []string) error {
	var span oteltrace.Span
	ctx, span = otel.Tracer(trace.TraceName).Start(ctx, "ApplySync")
	defer span.End()

	if d.Driver == "postgres" {
		return postgres.DeployPostgresStatements(d.URI, statements)
	} else if d.Driver == "mysql" {
		return mysql.DeployMysqlStatements(ctx, d.URI, statements)
	} else if d.Driver == "cockroachdb" {
		return postgres.DeployPostgresStatements(d.URI, statements)
	} else if d.Driver == "cassandra" {
		return cassandra.DeployCassandraStatements(d.Hosts, d.Username, d.Password, d.Keyspace, statements)
	} else if d.Driver == "sqlite" {
		return sqlite.DeploySqliteStatements(d.URI, statements)
	} else if d.Driver == "rqlite" {
		return rqlite.DeployRqliteStatements(d.URI, statements)
	} else if d.Driver == "timescaledb" {
		return postgres.DeployPostgresStatements(d.URI, statements)
	}

	return errors.Errorf("unknown database driver: %q", d.Driver)
}
