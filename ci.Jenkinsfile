loader.loadFrom('pipelines': [common          : 'common',
                              harbor          : 'flex/harbor',
                              devkit          : 'infra/devkit',
                              docker          : 'infra/docker',
                              custom_packaging: 'packaging/custom_packaging'])

devkit.devkitDockerImageTag = "latest"
devkit.dockerRegistry = common.DOCKER_REGISTRY.ASDREPO_ECS_REGISTRY

this.runTests()

void runTests() {
    currentBuild.description = ''
    Map<String, Object> args = [
            csiVersion    : params.CSI_VERSION,
            runMode       : '',
            slackChannel  : '',
            dockerProject : 'atlantic',
    ]

    String csiVersion = args.csiVersion
    final String RUN_MODE_MASTER = 'master'
    final String RUN_MODE_CUSTOM = 'custom'
    boolean testResultSuccess = false
    final String registry = "10.244.120.194:8085/atlantic"  // asdrepo.isus.emc.com:8085/atlantic
    common.node(label: 'ubuntu_build_hosts', time: 180) {
        try {

            stage('Start Minikube') {
                sh("""
                    minikube start --vm-driver=none --kubernetes-version=1.13.5
                """)
            }

            stage('Prepare iscsi') {
                sh("""
                   docker run --name itt -d --net=host ${registry}/itt:latest itt -p 127.0.0.1 -t 127.0.0.1:5230 -d /opt/emc/etc/itt/dev_ecs-test.xml
                   service iscsid start
                   iscsiadm --mode discovery --type=sendtargets --portal 127.0.0.1
                   iscsiadm --mode node --portal 127.0.0.1:3260 --login
                """)
            }
            common.withInfraDevkitContainerKind() {
                stage('Git clone') {
                    scmData = checkout scm
                    currentBuild.description += "GIT_BRANCH = ${scmData.GIT_BRANCH} <br>"
                    currentBuild.description += "CSI version: <b>${csiVersion}</b>"
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

                stage('Get fingerpint') {
                    custom_packaging.fingerprintVersionFile('bare-metal-csi', csiVersion)
                }

                //E2E tests can't work with helm, so we need to provide prepared yaml files for it
                stage('Prepare YAML for e2e tests') {
                    sh("helm template charts/baremetal-csi-plugin --output-dir /tmp --set image.tag=${csiVersion} " +
                            "--set global.registry=${registry} --set image.pullPolicy=IfNotPresent " +
                            "--set hwmgr.halOverride.iscsi=true")
                }

                stage('E2E testing') {
                    sh('''
                        kubectl apply -f charts/baremetal-csi-plugin/crds/availablecapacity.dell.com_availablecapacities.yaml
                        kubectl apply -f charts/baremetal-csi-plugin/crds/volume.dell.com_volumes.yaml 
                    ''')
                    String output = sh(script: 'go run test/e2e/baremetal_e2e.go -ginkgo.v -ginkgo.progress --kubeconfig=/root/.kube/config', returnStdout: true)
                    println(output)
                    if (!(output.contains("FAIL"))) {
                        testResultSuccess = true
                    }
                }
            }

            stage('Image retagging') {
                if (args.runMode != RUN_MODE_CUSTOM && testResultSuccess) {
                    harbor.retagCSIImages(args.dockerProject, csiVersion, 'latest')
                    common.withInfraDevkitContainerKind() {
                        List<String> repos = ["baremetal-csi-plugin-node",
                                              "baremetal-csi-plugin-controller",
                                              "baremetal-csi-plugin-hwmgr"]
                        // retag in asdrepo
                        repos.each { String repo ->
                            String image = "${registry}/${repo}"

                            sh("""
                                docker pull ${image}:${csiVersion}
                                docker tag ${image}:${csiVersion} ${image}:latest
                                docker push ${image}:latest
                            """)
                        }
                    }
                } else {
                    println('Skip pushing Docker images...')
                }
            }
        }
        catch (any) {
            println any
            common.setBuildFailure()
            throw any
        }
        finally {
            if (!testResultSuccess) {
                common.setBuildFailure()
            }
            sh("""
               iscsiadm --mode node --portal 127.0.0.1:3260 --logout
               iscsiadm --mode node --portal 127.0.0.1:3260 --op delete
               docker rm -f itt
               minikube delete
            """)
            common.slackSend(channel: args.slackChannel)
        }
    }
}

this
