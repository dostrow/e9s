package views

// Row accessors for the command palette's "go to resource" search. RowNames
// lists the rows a list view currently shows (after its filter), in display
// order, and WithCursor selects one of those rows by index.

func (m ClusterListModel) RowNames() []string {
	rows := m.filteredClusters()
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Name
	}
	return names
}

func (m ServiceListModel) RowNames() []string {
	rows := m.filteredServices()
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Name
	}
	return names
}

func (m ServiceListModel) WithCursor(i int) ServiceListModel { m.cursor = i; return m }

// ClusterName is the cluster whose services the list shows.
func (m ServiceListModel) ClusterName() string { return m.clusterName }

func (m SQSQueuesModel) RowNames() []string {
	rows := m.filteredQueues()
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Name
	}
	return names
}

func (m SQSQueuesModel) WithCursor(i int) SQSQueuesModel { m.cursor = i; return m }

func (m LambdaListModel) RowNames() []string {
	rows := m.filteredFunctions()
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Name
	}
	return names
}

func (m LambdaListModel) WithCursor(i int) LambdaListModel { m.cursor = i; return m }

func (m S3BucketsModel) RowNames() []string {
	rows := m.filteredBuckets()
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Name
	}
	return names
}

func (m S3BucketsModel) WithCursor(i int) S3BucketsModel { m.cursor = i; return m }

func (m DynamoTablesModel) RowNames() []string {
	return append([]string(nil), m.filteredTables()...)
}

func (m DynamoTablesModel) WithCursor(i int) DynamoTablesModel { m.cursor = i; return m }

func (m LogGroupsModel) RowNames() []string {
	rows := m.filteredGroups()
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Name
	}
	return names
}

func (m LogGroupsModel) WithCursor(i int) LogGroupsModel { m.cursor = i; return m }

func (m AlarmsModel) RowNames() []string {
	rows := m.filteredAlarms()
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Name
	}
	return names
}

func (m AlarmsModel) WithCursor(i int) AlarmsModel { m.cursor = i; return m }

func (m CBProjectsModel) RowNames() []string {
	rows := m.filteredProjects()
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Name
	}
	return names
}

func (m CBProjectsModel) WithCursor(i int) CBProjectsModel { m.cursor = i; return m }

func (m EC2InstancesModel) RowNames() []string {
	rows := m.filteredInstances()
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.InstanceID
		if r.Name != "" {
			names[i] = r.Name + " (" + r.InstanceID + ")"
		}
	}
	return names
}

func (m EC2InstancesModel) WithCursor(i int) EC2InstancesModel { m.cursor = i; return m }

func (m ECRReposModel) RowNames() []string {
	rows := m.filteredRepos()
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Name
	}
	return names
}

func (m ECRReposModel) WithCursor(i int) ECRReposModel { m.cursor = i; return m }

func (m R53ZonesModel) RowNames() []string {
	rows := m.filteredZones()
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Name
	}
	return names
}

func (m R53ZonesModel) WithCursor(i int) R53ZonesModel { m.cursor = i; return m }

func (m RDSClustersModel) RowNames() []string {
	rows := m.filteredClusters()
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Identifier
	}
	return names
}

func (m RDSClustersModel) WithCursor(i int) RDSClustersModel { m.cursor = i; return m }

func (m RDSInstancesModel) RowNames() []string {
	rows := m.filteredInstances()
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Identifier
	}
	return names
}

func (m RDSInstancesModel) WithCursor(i int) RDSInstancesModel { m.cursor = i; return m }

func (m SecretsModel) RowNames() []string {
	rows := m.filteredSecrets()
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Name
	}
	return names
}

func (m SecretsModel) WithCursor(i int) SecretsModel { m.cursor = i; return m }

func (m SSMModel) RowNames() []string {
	rows := m.filteredParams()
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Name
	}
	return names
}

func (m SSMModel) WithCursor(i int) SSMModel { m.cursor = i; return m }
