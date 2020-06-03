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
                                "--set drivemgr.type=LOOPBACK " +
                                "--set image.pullPolicy=IfNotPresent " +
                                "--set drivemgr.deployConfig=true")
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
                            kubectl apply -f charts/baremetal-csi-plugin/crds/baremetal-csi.dellemc.com_volumes.yaml
                            kubectl apply -f charts/baremetal-csi-plugin/crds/baremetal-csi.dellemc.com_drives.yaml
                            kubectl apply -f charts/baremetal-csi-plugin/crds/baremetal-csi.dellemc.com_lvgs.yaml
                            kubectl apply -f charts/baremetal-csi-plugin/templates/csidriver.yaml
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
                    harbor.retagCSIImages(args.dockerProject, csiVersion, 'latest')
                    common.withInfraDevkitContainerKind() {
                        List<String> repos = ["baremetal-csi-plugin-node",
                                              "baremetal-csi-plugin-controller",
                                              "baremetal-csi-plugin-drivemgr"]
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
            stage('Push artifacts to artifactory') {
                 if (args.runMode != RUN_MODE_CUSTOM && testResultSuccess) {
                    load('.ci/pipeline_variables.groovy')
                    common.withInfraDevkitContainerKind() {
                        this.publishCSIArtifactsToArtifactory([
                            version: csiVersion,
                        ])
                    }
                     permalink = "${ARTIFACTORY_ATLANTIC_DIR_PATH}/${COMPONENT_NAME}/latest"
                     artifactRepo = "${ARTIFACTORY_ATLANTIC_DIR_PATH}/${COMPONENT_NAME}/${csiVersion}"
                     common.publishPermalinkToArtifactory(permalink, artifactRepo, ARTIFACTORY_NAME)
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

private String getArtifactsJson(final Map<String, Object> args) {
    List<Object> artifacts = []
    List<String>  images = [
            "baremetal-csi-plugin-node",
            "baremetal-csi-plugin-drivemgr",
            "baremetal-csi-plugin-controller",
    ]
    images.each { image ->
        artifacts.add([
                "componentName": COMPONENT_NAME,
                "version": args.version,
                "type": "docker-image",
                "endpoint": "{{ ASD_REGISTRY }}",
                "path": "atlantic/${image}"
        ])
    }
    artifacts.add([
            "componentName": COMPONENT_NAME,
            "version": ATTACHER_VERSION,
            "type": "docker-image",
            "endpoint": "{{ ASD_REGISTRY }}",
            "path": "atlantic/csi-attacher"

    ])
    artifacts.add([
            "componentName": COMPONENT_NAME,
            "version": PROVISION_VERSION,
            "type": "docker-image",
            "endpoint": "{{ ASD_REGISTRY }}",
            "path": "atlantic/csi-provisioner"
    ])
    artifacts.add([
            "componentName": COMPONENT_NAME,
            "version": NODE_REGISTRAR_VERSION,
            "type": "docker-image",
            "endpoint": "{{ ASD_REGISTRY }}",
            "path": "atlantic/csi-node-driver-registrar",
    ])
    artifacts.add([
            "componentName": COMPONENT_NAME,
            "version": args.version,
            "type": "helm-chart",
            "endpoint": "{{ ASD_REPO }}",
            "path": args.chartsPath
    ])
    return common.toJSON(["componentVersion": args.version, "componentArtifacts": artifacts], true)
}

void publishCSIArtifactsToArtifactory(final Map<String, Object> args) {
    final String chartsBuildPath = "build/charts"
    sh("""
        helm package charts/baremetal-csi-plugin/ --set image.tag=${args.version} --version ${args.version} --destination ${chartsBuildPath}
    """)
    file = common.findFiles("${chartsBuildPath}/*.tgz")[0]
    final String name = file.getName()
    final String remoteName = file.getRemote()
    final String chartsPathToPublish = "${ARTIFACTORY_FULL_CHARTS_PATH}/${args.version}"
    common.publishFileToArtifactory(remoteName, chartsPathToPublish, common.ARTIFACTORY.ATLANTIC_PUBLISH_CREDENTIALS_ID)
    final String text = this.getArtifactsJson([
            version: args.version,
            chartsPath: "${ARTIFACTORY_CHARTS_PATH}/${args.version}/${name}"
    ])

    writeFile(file: "artifacts.json",
            text: text)

    final String pathToPublish = "${ARTIFACTORY_COMPONENT_PATH}/${args.version}"
    common.publishFileToArtifactory("artifacts.json", pathToPublish, common.ARTIFACTORY.ATLANTIC_PUBLISH_CREDENTIALS_ID)
}

this
