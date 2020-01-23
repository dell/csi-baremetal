// load pipelines libraries from https://eos2git.cec.lab.emc.com/ECS/pipelines
loader.loadFrom('pipelines': [common       : 'common',
                              pr_validation: 'infra/pr_validation',])

// run this job
this.runPullRequestValidationJob()

// define this job
void runPullRequestValidationJob() {
    Map<String, Object> args = [
            pullRequestNumber: params.PULL_REQUEST_NUMBER,
            repo             : common.BAREMETAL_CSI_PLUGIN_REPO_SSH,
    ]
    pr_validation.runPullRequestValidationJob(args) {
        String commit ->
            return this.validatePullRequest(commit)
    }
}

// define pr-validation logic
boolean validatePullRequest(String commit) {
    int lintExitCode = 0
    int testExitCode = 0
    int coverageExitCode = 0
    int buildExitCode = 0
    int imageExitCode = 0
    common.node(common.JENKINS_LABELS.FLEX_CI) {
        /*
        * IMPORTANT: all sh() commands must be performed from common.withInfraDevkitContainer() block
        */
        common.withInfraDevkitContainer() {
            try {
                stage('Git Clone') {
                    checkout scm
                }

                stage('Get dependencies') {
                    depExitCode = sh(script: '''
                                        make install-compile-proto
                                        make install-hal
                                        make dependency
                                     ''', returnStatus: true)
                    if (depExitCode != 0) {
                        currentBuild.result = 'FAILURE'
                        throw new Exception("Get dependencies stage failed, check logs")
                    }
                }

                stage('Lint') {
                    lintExitCode = sh(script: 'make lint', returnStatus: true)
                    if (lintExitCode != 0) {
                        currentBuild.result = 'FAILURE'
                        throw new Exception("Lint stage failed, check logs")
                    }
                }

                stage('Build') {
                    buildExitCode = sh(script: 'make build', returnStatus: true)
                    if (buildExitCode != 0) {
                        currentBuild.result = 'FAILURE'
                        throw new Exception("Build stage failed, check logs")
                    }
                }

                stage('Test and Coverage') {
                    testExitCode = sh(script: 'make test', returnStatus: true)
                    //split because our make test fails and make coverage isn't invoked during sh()
                    coverageExitCode = sh(script: 'make coverage', returnStatus: true)
                    if ((testExitCode != 0) || (coverageExitCode != 0)) {
                        currentBuild.result = 'FAILURE'
                        throw new Exception("Test and Coverage stage failed, check logs")
                    }
                }

                stage('Make image') {
                    imageExitCode = sh(script: 'make image', returnStatus: true)
                    if (imageExitCode != 0) {
                        currentBuild.result = 'FAILURE'
                        throw new Exception("Image stage failed, check logs")
                    }
                }
            } finally {
                // publish in Jenkins test results
                archiveArtifacts('coverage.html')
            }
        }
    }
    // If we got here then nothing failed
    return true // as a mark of successful validation
}

this
