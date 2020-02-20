import com.emc.pipelines.docker.DockerRegistries
import com.emc.pipelines.docker.DockerImage

loader.loadFrom('pipelines': [common          : 'common',
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
            runMode      : '',
            slackChannel : '',
            harborProject: 'ecs',
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
                    currentBuild.description += "GIT_BRANCH = ${scmData.GIT_BRANCH} <br>"
                    currentBuild.description += "CSI version: <b>${csiTag}</b>"
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
                    harbor.retagCSIImages(args.harborProject, args.csiTag, 'latest')
                    String repo = 'baremetal-csi-plugin'
                    DockerImage sourceImage = new DockerImage(registry: registry, repo: repo, tag: args.csiTag)
                    DockerImage newImage = new DockerImage(registry: registry, repo: repo, tag: 'latest')
                    sh(docker.getRetagCommand(sourceImage, newImage))
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
