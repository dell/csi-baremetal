import com.emc.pipelines.docker.DockerRegistries

loader.loadFrom('pipelines': [common          : 'common',
                              docker_helper   : 'flex/docker_helper',
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
    ]

    String csiVersion = args.csiVersion
    final String RUN_MODE_MASTER = 'master'
    final String RUN_MODE_CUSTOM = 'custom'
    boolean testResultSuccess = false
    final String registry = common.DOCKER_REGISTRY.ASDREPO_ATLANTIC_REGISTRY.getRegistryUrl()
    common.node(label: 'csi_test', time: 180) {
        try {
            common.withInfraDevkitContainerKind() {
                try {
                    stage('Git clone') {
                        scmData = checkout scm
                        currentBuild.description += "GIT_BRANCH = ${scmData.GIT_BRANCH} <br>"
                        currentBuild.description += "CSI version: <b>${csiVersion}</b>"
                        args.runMode = (scmData.GIT_BRANCH == 'origin/master') ? RUN_MODE_MASTER : RUN_MODE_CUSTOM
                        if (args.runMode == RUN_MODE_MASTER) {
                            args += [
                                    slackChannel: common.SLACK_CHANNEL.ECS_BARE_METAL_CSI,
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

                    stage('Get dependencies') {
                        sh("make install-compile-proto")
                    }
                    //E2E tests can't work with helm, so we need to provide prepared yaml files for it
                    stage('Prepare YAML for e2e tests') {
                        sh("helm template charts/baremetal-csi-plugin --output-dir /tmp --set image.tag=${csiVersion} " +
                                "--set env.test=true " +
                                "--set drivemgr.type=loopbackmgr " +
                                "--set drivemgr.deployConfig=true " +
                                "--set image.pullPolicy=IfNotPresent")
						sh("helm template charts/scheduler-extender --output-dir /tmp --set image.tag=${csiVersion} " +
								"--set env.test=true " +
								"--set image.pullPolicy=IfNotPresent " +
								"--set patcher.enable=true " +
								"--set patcher.restore_on_shutdown=true")
                    }
                    // for LVM tests we need to use custom kind version with --ipc-host for worker nodes
                    // todo - upstream custom changes - https://jira.cec.lab.emc.com:8443/browse/AK8S-1186
                    stage('Install custom kind version') {
                        sh ("""
                          wget -O kind http://big-bang.lss.emc.com/export/home/borism1/bin/kind/host-ipc/kind_v0.8.1
                          chmod +x kind
                          mv kind /usr/bin
                        """)
                    }
                    stage('Start Kind') {
                        sh("""
                          kind create cluster --config test/kind/kind.yaml
                        """)
                    }
                    stage('Prepare images for Kind') {
                        sh("""
                          make kind-pull-images TAG=${csiVersion} REGISTRY=${registry}
                          make kind-tag-images TAG=${csiVersion} REGISTRY=${registry}
                          make kind-load-images TAG=${csiVersion} REGISTRY=${registry}
                        """)
                    }

                    stage('E2E testing') {
                        sh('''
                            kubectl apply -f charts/baremetal-csi-plugin/crds/baremetal-csi.dellemc.com_availablecapacities.yaml
                            kubectl apply -f charts/baremetal-csi-plugin/crds/baremetal-csi.dellemc.com_availablecapacityreservations.yaml
                            kubectl apply -f charts/baremetal-csi-plugin/crds/baremetal-csi.dellemc.com_volumes.yaml
                            kubectl apply -f charts/baremetal-csi-plugin/crds/baremetal-csi.dellemc.com_drives.yaml
                            kubectl apply -f charts/baremetal-csi-plugin/crds/baremetal-csi.dellemc.com_lvgs.yaml
                            kubectl apply -f /tmp/baremetal-csi-plugin/templates/csidriver.yaml
                        ''')
                        testExitCode = sh(script: "make test-ci", returnStatus: true)
                        archiveArtifacts('log.txt')
                        common.parseJunitResults(searchPattern: 'test/e2e/report.xml')
                        if ((testExitCode == 0)) {
                            testResultSuccess = true
                        }
                    }
                } finally {
                    sh("""
                        kind delete cluster
                      """)
                }
            }
            stage('Images re-tagging') {
                if (args.runMode != RUN_MODE_CUSTOM && testResultSuccess) {
                    docker_helper.retagCSIImages(csiVersion, 'latest', DockerRegistries.ASDREPO_ATLANTIC_REGISTRY)
                    common.withInfraDevkitContainerKind() {
                        List<String> repos = ["baremetal-csi-plugin-node",
                                              "baremetal-csi-plugin-controller",
                                              "baremetal-csi-plugin-basemgr",
                                              "baremetal-csi-plugin-halmgr",
                                              "baremetal-csi-plugin-loopbackmgr",
                                              "baremetal-csi-plugin-extender",
                                              "baremetal-csi-plugin-scheduler-patcher",
                                              "baremetal-csi-plugin-scheduler"]
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
            stage('Promote artifacts') {
                 if (args.runMode != RUN_MODE_CUSTOM && testResultSuccess) {
                    load('.ci/pipeline_variables.groovy')
                    if (!(common.checkManifest(ARTIFACTORY_COMPONENT_PATH, csiVersion))) {
                        println('Manifest check failed')
                        common.setBuildFailure()
                    } else {
                        permalink = "${ARTIFACTORY_ATLANTIC_DIR_PATH}/${COMPONENT_NAME}/latest"
                        artifactRepo = "${ARTIFACTORY_ATLANTIC_DIR_PATH}/${COMPONENT_NAME}/${csiVersion}"
                        common.publishPermalinkToArtifactory(permalink, artifactRepo, ARTIFACTORY_NAME)
                    }
                } else {
                    println('Skip pushing artifacts..')
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
            common.wipeDockerContainers()
            common.wipeDockerImages()
            common.slackSend(channel: args.slackChannel)
        }
    }
}

this
