package main

// Известные записи, отдаются без обращения к upstream.

var KnownResources = []KnownResource{
	{
		ResourceId{[]string{"ya", "ru"}, RESOURCE_TYPE_A, CLASS_INTERNET},
		24 * 60 * 60,
		RecordA{ipv4: "127.0.0.1"},
	},
}

// lookupKnownResource возвращает известную запись, соответствующую вопросу.
func lookupKnownResource(question ResourceId) (KnownResource, bool) {
	for _, known := range KnownResources {
		if equalResourceIds(known.ResourceId, question) {
			return known, true
		}
	}

	return KnownResource{}, false
}
