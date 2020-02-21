import com.emc.pipelines.docker.DockerRegistries
import com.emc.pipelines.docker.DockerImage

loader.loadFrom('pipelines': [common          : 'common',
                              custom_packaging: 'packaging/custom_packaging',
                              harbor          : 'flex/harbor',
                              devkit          : 'infra/devkit',
                              docker          : 'infra/docker',])

devkit.devkitDockerImageTag = "latest"
devkit.dockerRegistry = common.DOCKER_REGISTRY.ASDREPO_ECS_REGISTRY

this.runTests()

void runTests() {
    currentBuild.description = ''

    Map<String, Object> args = [
            csiTag       : params.CSI_TAG,
            version      : '',
            runMode      : '',
            slackChannel : '',
            harborProject: 'atlantic',
    ]
    String csiTag = args.csiTag
    final String RUN_MODE_MASTER = 'master'
    final String RUN_MODE_CUSTOM = 'custom'
    final String registry = "10.244.120.194:8085/atlantic"  // asdrepo.isus.emc.com:8085/atlantic
    try {
        common.node(label: common.JENKINS_LABELS.FLEX_CI, time: 180) {
            String workspace = pwd()
            common.withInfraDevkitContainerKind() {
                stage('Git clone') {
                    scmData = checkout scm
                    args.runMode = (scmData.GIT_BRANCH == 'origin/master') ? RUN_MODE_MASTER : RUN_MODE_CUSTOM
                    if (args.runMode == RUN_MODE_MASTER) {
                        args += [
                                slackChannel: common.SLACK_CHANNEL.ECS_BARE_METAL_K8S_CI,
                        ]
                    } else if (args.runMode == RUN_MODE_CUSTOM) {
                        args += [
                                slackChannel: common.SLACK_CHANNEL.ECS_DEV,
                        ]
                    }
                }

                stage('Get Version') {
                    args.version = common.getMakefileVar('FULL_VERSION')
                    currentBuild.description += "CSI version: <b>${args.version}</b>"
                    custom_packaging.fingerprintVersionFile('bare-metal-csi', args.version)
                }

                stage('Get dependencies') {
                    depExitCode = sh(script: '''
                                        make install-compile-proto
                                        make install-hal
                                        make install-controller-gen
                                        make generate-deepcopy
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
            }
            stage('Image retagging') {
                if (args.runMode != RUN_MODE_MASTER) {
                    // retag in harbor
                    harbor.retagCSIImages(args.harborProject, args.csiTag, 'latest')

                    components = ['baremetal-csi-plugin-node',
                                  'baremetal-csi-plugin-controller',
                                  'baremetal-csi-plugin-hwmgr']
                    // retag in asdrepo
                    for (String component: components) {
                        DockerImage sourceImage = new DockerImage(registry: registry, repo: component, tag: args.csiTag)
                        DockerImage newImage = new DockerImage(registry: registry, repo: component, tag: 'latest')
                        sh(docker.getRetagCommand(sourceImage, newImage))
                    }
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
