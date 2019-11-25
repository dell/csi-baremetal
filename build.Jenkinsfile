// load pipeline libraries
loader.loadFrom('pipelines': [common          : 'common',
                              custom_packaging: 'packaging/custom_packaging'])
// run this job
this.runJob()

void runJob() {
    // pipeline workflow data
    Map<String, Object> args = [
        runMode                      : '',
        bareMetalCsiRepoBranchName   : '',
        version                      : '',
        slackChannel                 : '',
    ]
    // possible run modes
    final String RUN_MODE_MASTER = 'master'
    final String RUN_MODE_CUSTOM = 'custom'
    // build description
    currentBuild.description = ''

    try {
        common.node(label: common.JENKINS_LABELS.FLEX_CI, time: 180) {
            /*
             * IMPORTANT: all sh() commands must be performed from common.withInfraDevkitContainer() block
             */
            common.withInfraDevkitContainer() {

                stage('Git clone') {
                    scmData = checkout scm
                    currentBuild.description += "GIT_BRANCH = ${scmData.GIT_BRANCH} <br>"
                    
                    // Identify run mode
                    args.runMode = (scmData.GIT_BRANCH == 'origin/master') ? RUN_MODE_MASTER : RUN_MODE_CUSTOM

                    if (args.runMode == RUN_MODE_MASTER) {
                        args += [
                            bareMetalCsiRepoBranchName: 'master',
                            slackChannel              : common.SLACK_CHANNEL.ECS_BARE_METAL_K8S_CI,
                        ]
                    } else if (args.runMode == RUN_MODE_CUSTOM) {
                        args += [
                            bareMetalCsiRepoBranchName: params.BRANCH,
                            slackChannel              : common.SLACK_CHANNEL.ECS_DEV,
                        ]
                    }
                }

                stage('Get Version') {
                    args.version = common.getMakefileVar('FULL_VERSION')
                    currentBuild.description += "CSI version: <b>${args.version}</b>"
                    custom_packaging.fingerprintVersionFile('bare-metal-csi', args.version)
                }

                stage('Lint') {
                    sh('make lint')
                }

                stage('Test and Coverage') {
                    sh('''
                       make test
                       make coverage
                       ''')
                }

                stage('Build') {
                    sh('make build')
                }

                stage('Image') {
                    sh("make image")
                }

                stage('Push image') {
                    sh("make push")
                }

                if (args.runMode != RUN_MODE_CUSTOM) {
                    build([
                        job       : "csi-${args.bareMetalCsiRepoBranchName}-ci".toString(),
                        parameters: [string(name: 'CSI_VERSION', value: args.version)],
                        wait      : false,
                        propagate : false,
                    ])
                }
            }
        }
    }
    catch (any) {
        println any
        common.setBuildFailure()
        throw any
    }
    finally {
        common.slackSend(channel: args.slackChannel)
    }
}

this
