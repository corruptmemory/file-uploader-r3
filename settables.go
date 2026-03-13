package main

var (
	PublicAPIEnvironment      string
	PublicAPIEndpoint         string
	ConfigVersion             string
	GitSHA                    string = "<unknown>"
	DirtyBuild                string = "<unknown>"
	GitFullVersionDescription string = "<unknown>"
	GitDescribeVersion        string = "<unknown>"
	GitLastCommitDate         string = "<unknown>"
	GitVersion                string = "<unknown>"
	SnapshotBuild             string = "<unknown>"
)

func PrintVersion() {
	println("Version: " + GitVersion)
	println("GitSHA: " + GitSHA)
	println("GitFullVersionDescription: " + GitFullVersionDescription)
	println("GitDescribeVersion: " + GitDescribeVersion)
	println("GitLastCommitDate: " + GitLastCommitDate)
	println("SnapshotBuild: " + SnapshotBuild)
	println("DirtyBuild: " + DirtyBuild)
	println("ConfigVersion: " + ConfigVersion)
	println("PublicAPIEnvironment: " + PublicAPIEnvironment)
	println("PublicAPIEndpoint: " + PublicAPIEndpoint)
}
