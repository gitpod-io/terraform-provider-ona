# Naming Conventions

Consistent naming is what keeps a provider navigable and prevents the singular/plural confusion that bites in long file lists and registration slices.

## Type names (the part users see, with a real convention)

Provider-prefixed, singular for a get, plural for a list:

```
foo_widget     foo_widgets
foo_runner     foo_runners
foo_project    foo_projects
```

The plural is a real English plural (`foo_policies`, not `foo_policy_list` or `foo_widgets_data`). The pluralization carries the meaning; do not add `_list`, `_all`, or `_plural` suffixes to the type name. This is the one naming decision users and any future List Resources work depend on, so keep it the clean plural regardless of how the Go identifiers are named.

## Go identifiers: the Collection convention

The framework imposes nothing on Go names, so pick one scheme and hold to it. The bare-plural scheme (`widgetDataSource` vs `widgetsDataSource`) is idiomatic but relies on a single trailing `s`, which is easy to misread. When disambiguation matters, use the **Collection** convention: the singular stays the bare noun, the plural gets an explicit `Collection` qualifier, and the user-facing type name stays the clean plural underneath.

| User-facing type | Go type | Constructor |
|---|---|---|
| `foo_organization` | `OrganizationDataSource` | `NewOrganizationDataSource` |
| `foo_organizations` | `OrganizationCollectionDataSource` | `NewOrganizationCollectionDataSource` |
| `foo_runner` | `RunnerDataSource` | `NewRunnerDataSource` |
| `foo_runners` | `RunnerCollectionDataSource` | `NewRunnerCollectionDataSource` |
| `foo_project` | `ProjectDataSource` | `NewProjectDataSource` |
| `foo_projects` | `ProjectCollectionDataSource` | `NewProjectCollectionDataSource` |

Full identifier set per pair:

```go
// organization_data_source.go
type OrganizationDataSource struct{ client *api.Client }
type OrganizationDataSourceModel struct { /* one org */ }
func NewOrganizationDataSource() datasource.DataSource { return &OrganizationDataSource{} }
func (d *OrganizationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
    resp.TypeName = req.ProviderTypeName + "_organization"
}

// organization_collection_data_source.go
type OrganizationCollectionDataSource struct{ client *api.Client }
type OrganizationCollectionDataSourceModel struct {
    Organizations []OrganizationModel `tfsdk:"organizations"`
    // filter inputs
}
func NewOrganizationCollectionDataSource() datasource.DataSource { return &OrganizationCollectionDataSource{} }
func (d *OrganizationCollectionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
    resp.TypeName = req.ProviderTypeName + "_organizations" // user still sees the clean plural
}
```

## The naming rules that keep this mechanical

- Singular is always the bare noun: `<Noun>DataSource`.
- Plural is always `<Noun>CollectionDataSource`. Keep the noun **singular** inside the identifier; `Collection` already conveys plurality, so `OrganizationCollectionDataSource` (not `OrganizationsCollectionDataSource`). The only places the real plural appears are the user-facing `_organizations` type name and the `Organizations []OrganizationModel` field, which genuinely holds many.
- Files mirror the type: `organization_data_source.go`, `organization_collection_data_source.go`.
- Models mirror it: `OrganizationModel` for the shared per-item shape, plus `OrganizationDataSourceModel` and `OrganizationCollectionDataSourceModel` for the two read targets.
- Apply the convention uniformly across every type; never mix bare-plural and Collection in the same provider.

Registration reads cleanly and is impossible to confuse at the call site:

```go
func (p *fooProvider) DataSources(_ context.Context) []func() datasource.DataSource {
    return []func() datasource.DataSource{
        NewOrganizationDataSource, NewOrganizationCollectionDataSource,
        NewRunnerDataSource, NewRunnerCollectionDataSource,
        NewProjectDataSource, NewProjectCollectionDataSource,
    }
}
```

## Reserve "List" for the List Resources primitive

Do not name a plural data source with `List`/`ListResource`. Those identifiers belong to the actual List Resources feature (`advanced-primitives.md`), and reusing them invites exactly the confusion the Collection convention removes. "Singular" and "plural" are descriptive shorthand, not an official taxonomy; under the hood both are ordinary data sources. "List resource" is a formally named, distinct primitive. Keep the vocabularies separate.
