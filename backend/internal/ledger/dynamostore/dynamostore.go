// Package dynamostore keeps the ledger in one DynamoDB table.
//
// Layout (the table itself is created by infra/terraform/modules/data; CreateTable is the same schema
// for DynamoDB Local and tests, keep the two in sync):
//
//	pk     sk          item
//	TX     <id>        a transaction, plus ts = its time in UTC for the by_time index
//	RULE   <id>        a recurring rule
//	META   categories  the category list
//	META   accounts    the account list
//
// The local secondary index by_time (pk, ts) answers "transactions between two instants". A local
// index shares the table's capacity, so it adds nothing to the free-tier budget.
package dynamostore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"finance/internal/ledger"
	"finance/internal/transaction"
)

const (
	pkTx         = "TX"
	pkRule       = "RULE"
	pkMeta       = "META"
	skCategories = "categories"
	skAccounts   = "accounts"
	indexByTime  = "by_time"
)

// tsLayout is fixed width and always UTC, so sorting the text sorts by time.
const tsLayout = "2006-01-02T15:04:05.000000000Z"

func ts(t time.Time) string { return t.UTC().Format(tsLayout) }

// Items reuse the API's json tags as attribute names.
func encode(v any) (map[string]types.AttributeValue, error) {
	return attributevalue.MarshalMapWithOptions(v, func(o *attributevalue.EncoderOptions) { o.TagKey = "json" })
}

func decode(m map[string]types.AttributeValue, v any) error {
	return attributevalue.UnmarshalMapWithOptions(m, v, func(o *attributevalue.DecoderOptions) { o.TagKey = "json" })
}

type txItem struct {
	PK string `json:"pk"`
	SK string `json:"sk"`
	TS string `json:"ts"`
	transaction.Transaction
}

type ruleItem struct {
	PK string `json:"pk"`
	SK string `json:"sk"`
	ledger.Rule
	Through string `json:"through,omitempty"` // ledger.Rule hides it from the API
}

type categoriesItem struct {
	PK    string            `json:"pk"`
	SK    string            `json:"sk"`
	Items []ledger.Category `json:"items"`
}

type accountsItem struct {
	PK    string           `json:"pk"`
	SK    string           `json:"sk"`
	Items []ledger.Account `json:"items"`
}

// Store implements ledger.Store.
type Store struct {
	db    *dynamodb.Client
	table string
}

func New(db *dynamodb.Client, table string) *Store { return &Store{db: db, table: table} }

func key(pk, sk string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: pk},
		"sk": &types.AttributeValueMemberS{Value: sk},
	}
}

// conditionFailed reports whether a write was refused by its condition expression.
func conditionFailed(err error) bool {
	var ccf *types.ConditionalCheckFailedException
	return errors.As(err, &ccf)
}

// get reads one item with a strongly consistent read: the dashboard reloads right after it writes.
func (s *Store) get(ctx context.Context, pk, sk string, v any) error {
	out, err := s.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &s.table, Key: key(pk, sk), ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return err
	}
	if out.Item == nil {
		return ledger.ErrNotFound
	}
	return decode(out.Item, v)
}

// put writes an item. condition, if set, must hold or the write fails with ErrDuplicate/ErrNotFound.
func (s *Store) put(ctx context.Context, item any, condition string) error {
	av, err := encode(item)
	if err != nil {
		return err
	}
	in := &dynamodb.PutItemInput{TableName: &s.table, Item: av}
	if condition != "" {
		in.ConditionExpression = &condition
	}
	_, err = s.db.PutItem(ctx, in)
	return err
}

func (s *Store) delete(ctx context.Context, pk, sk string) error {
	_, err := s.db.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &s.table, Key: key(pk, sk), ConditionExpression: aws.String("attribute_exists(pk)"),
	})
	if conditionFailed(err) {
		return ledger.ErrNotFound
	}
	return err
}

// query returns every item matching in, following pages.
func (s *Store) query(ctx context.Context, in *dynamodb.QueryInput) ([]map[string]types.AttributeValue, error) {
	in.TableName = &s.table
	in.ConsistentRead = aws.Bool(true)
	var out []map[string]types.AttributeValue
	for p := dynamodb.NewQueryPaginator(s.db, in); p.HasMorePages(); {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Items...)
	}
	return out, nil
}

func (s *Store) Categories(ctx context.Context) ([]ledger.Category, bool, error) {
	var it categoriesItem
	switch err := s.get(ctx, pkMeta, skCategories, &it); {
	case errors.Is(err, ledger.ErrNotFound):
		return nil, false, nil
	case err != nil:
		return nil, false, err
	}
	return it.Items, true, nil
}

func (s *Store) SaveCategories(ctx context.Context, list []ledger.Category) error {
	return s.put(ctx, categoriesItem{pkMeta, skCategories, list}, "")
}

func (s *Store) Accounts(ctx context.Context) ([]ledger.Account, error) {
	var it accountsItem
	if err := s.get(ctx, pkMeta, skAccounts, &it); err != nil && !errors.Is(err, ledger.ErrNotFound) {
		return nil, err
	}
	return it.Items, nil
}

func (s *Store) SaveAccounts(ctx context.Context, list []ledger.Account) error {
	return s.put(ctx, accountsItem{pkMeta, skAccounts, list}, "")
}

func (s *Store) TransactionsBetween(ctx context.Context, from, to time.Time) ([]transaction.Transaction, error) {
	if !from.Before(to) {
		return nil, nil
	}
	items, err := s.query(ctx, &dynamodb.QueryInput{
		IndexName:              aws.String(indexByTime),
		KeyConditionExpression: aws.String("pk = :pk AND ts BETWEEN :from AND :to"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":   &types.AttributeValueMemberS{Value: pkTx},
			":from": &types.AttributeValueMemberS{Value: ts(from)},
			// BETWEEN includes both ends; the last nanosecond before to makes the end exclusive.
			":to": &types.AttributeValueMemberS{Value: ts(to.Add(-time.Nanosecond))},
		},
	})
	if err != nil {
		return nil, err
	}
	out := make([]transaction.Transaction, 0, len(items))
	for _, m := range items {
		var it txItem
		if err := decode(m, &it); err != nil {
			return nil, err
		}
		out = append(out, it.Transaction)
	}
	return out, nil
}

func (s *Store) Transaction(ctx context.Context, id string) (transaction.Transaction, error) {
	var it txItem
	err := s.get(ctx, pkTx, id, &it)
	return it.Transaction, err
}

func txFor(t transaction.Transaction) txItem { return txItem{pkTx, t.ID, ts(t.OccurredAt), t} }

func (s *Store) InsertTransaction(ctx context.Context, t transaction.Transaction) error {
	err := s.put(ctx, txFor(t), "attribute_not_exists(pk)")
	if conditionFailed(err) {
		return ledger.ErrDuplicate
	}
	return err
}

func (s *Store) ReplaceTransaction(ctx context.Context, t transaction.Transaction) error {
	err := s.put(ctx, txFor(t), "attribute_exists(pk)")
	if conditionFailed(err) {
		return ledger.ErrNotFound
	}
	return err
}

func (s *Store) DeleteTransaction(ctx context.Context, id string) error {
	return s.delete(ctx, pkTx, id)
}

func (s *Store) Rules(ctx context.Context) ([]ledger.Rule, error) {
	items, err := s.query(ctx, &dynamodb.QueryInput{
		KeyConditionExpression:    aws.String("pk = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":pk": &types.AttributeValueMemberS{Value: pkRule}},
	})
	if err != nil {
		return nil, err
	}
	out := make([]ledger.Rule, 0, len(items))
	for _, m := range items {
		var it ruleItem
		if err := decode(m, &it); err != nil {
			return nil, err
		}
		it.Rule.Through = it.Through
		out = append(out, it.Rule)
	}
	return out, nil
}

func (s *Store) Rule(ctx context.Context, id string) (ledger.Rule, error) {
	var it ruleItem
	if err := s.get(ctx, pkRule, id, &it); err != nil {
		return ledger.Rule{}, err
	}
	it.Rule.Through = it.Through
	return it.Rule, nil
}

func (s *Store) PutRule(ctx context.Context, r ledger.Rule) error {
	return s.put(ctx, ruleItem{pkRule, r.ID, r, r.Through}, "")
}

func (s *Store) DeleteRule(ctx context.Context, id string) error {
	return s.delete(ctx, pkRule, id)
}

// CreateTable creates the table with the same schema as Terraform, for DynamoDB Local and tests.
func CreateTable(ctx context.Context, db *dynamodb.Client, table string) error {
	str := types.ScalarAttributeTypeS
	_, err := db.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName:   &table,
		BillingMode: types.BillingModePayPerRequest,
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: str},
			{AttributeName: aws.String("sk"), AttributeType: str},
			{AttributeName: aws.String("ts"), AttributeType: str},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
			{AttributeName: aws.String("sk"), KeyType: types.KeyTypeRange},
		},
		LocalSecondaryIndexes: []types.LocalSecondaryIndex{{
			IndexName: aws.String(indexByTime),
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String("ts"), KeyType: types.KeyTypeRange},
			},
			Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
		}},
	})
	if err != nil {
		return fmt.Errorf("creating table %s: %w", table, err)
	}
	return dynamodb.NewTableExistsWaiter(db).Wait(ctx, &dynamodb.DescribeTableInput{TableName: &table}, time.Minute)
}
