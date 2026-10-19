package openapi

// Documents partitions contracts into independent legacy/versioned documents.
func Documents(contracts []Contract) (map[string][]byte, error) {
	groups := map[string][]Contract{"": {}}
	for _, c := range contracts {
		groups[c.Version] = append(groups[c.Version], c)
	}
	documents := map[string][]byte{}
	for version, group := range groups {
		data, err := Generate(group)
		if err != nil {
			return nil, err
		}
		if err = Validate(data); err != nil {
			return nil, err
		}
		documents[version] = data
	}
	return documents, nil
}
