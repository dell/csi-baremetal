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
        common.node(label: 'ubuntu_build_hosts', time: 180) {
            /*
             * IMPORTANT: all sh() commands must be performed from common.withInfraDevkitContainer() block
             */
            // clean all images to use "latest" devkit image
            common.wipeDockerImages()
            common.withInfraDevkitContainer() {

                stage('Git clone') {
                    scmData = checkout scm
                    currentBuild.description += "GIT_BRANCH = ${scmData.GIT_BRANCH} <br>"
                    
                    // Identify run mode
                    args.runMode = (scmData.GIT_BRANCH == 'origin/master') ? RUN_MODE_MASTER : RUN_MODE_CUSTOM

                    if (args.runMode == RUN_MODE_MASTER) {
                        args += [
                            bareMetalCsiRepoBranchName: 'master',
                            slackChannel              : common.SLACK_CHANNEL.ECS_BARE_METAL_CSI,
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

                stage('Get Dependencies') {
                    sh('''
                        make install-compile-proto
                        make install-hal
                        make install-controller-gen
                        make generate-deepcopy
                        make dependency
                    ''')
                }

                stage('Build') {
                    sh('''
                        make DRIVE_MANAGER_TYPE=basemgr build
                        make DRIVE_MANAGER_TYPE=halmgr build-drivemgr
                        make DRIVE_MANAGER_TYPE=loopbackmgr build-drivemgr
                        make DRIVE_MANAGER_TYPE=idracmgr build-drivemgr
                    ''')
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

                stage('Base images') {
                    sh('''
                        make DRIVE_MANAGER_TYPE=basemgr base-images
                        make DRIVE_MANAGER_TYPE=loopbackmgr base-image-drivemgr
                        make DRIVE_MANAGER_TYPE=halmgr base-image-drivemgr
                    ''')
                }

                stage('Images') {
                    sh('''
                        make DRIVE_MANAGER_TYPE=basemgr images
                        make DRIVE_MANAGER_TYPE=loopbackmgr image-drivemgr
                        make DRIVE_MANAGER_TYPE=halmgr image-drivemgr
                    ''')
                }

                stage('Push images') {
                    sh("""
                        ${common.DOCKER_REGISTRY.DOCKER_HUB.getLoginCommand()}
                        make DRIVE_MANAGER_TYPE=basemgr push
                        make DRIVE_MANAGER_TYPE=loopbackmgr push-drivemgr
                        make DRIVE_MANAGER_TYPE=halmgr push-drivemgr
                    """)
                }

                stage('Push artifacts to artifactory') {
                    load('.ci/pipeline_variables.groovy')
                    common.withInfraDevkitContainerKind() {
                        this.publishCSIArtifactsToArtifactory([
                                version: args.version,
                        ])
                    }
                }

                if (args.runMode != RUN_MODE_CUSTOM) {
                    build([
                        job       : 'csi-master-ci',
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

private String getArtifactsJson(final Map<String, Object> args) {
    List<Object> artifacts = []
    List<String>  images = [
            "baremetal-csi-plugin-node",
            "baremetal-csi-plugin-controller",
            "baremetal-csi-plugin-halmgr",
            "baremetal-csi-plugin-basemgr",
            "baremetal-csi-plugin-loopbackmgr",
            "baremetal-csi-plugin-extender"
    ]
    images.each { image ->
        artifacts.add([
                "componentName": COMPONENT_NAME,
                "version": args.version,
                "type": "docker-image",
                "endpoint": "{{ ATLANTIC_REGISTRY }}",
                "path": "${image}"
        ])
    }
    artifacts.add([
            "componentName": COMPONENT_NAME,
            "version": ATTACHER_VERSION,
            "type": "docker-image",
            "endpoint": "{{ ATLANTIC_REGISTRY }}",
            "path": "csi-attacher"

    ])
    artifacts.add([
            "componentName": COMPONENT_NAME,
            "version": PROVISION_VERSION,
            "type": "docker-image",
            "endpoint": "{{ ATLANTIC_REGISTRY }}",
            "path": "csi-provisioner"
    ])
    artifacts.add([
            "componentName": COMPONENT_NAME,
            "version": NODE_REGISTRAR_VERSION,
            "type": "docker-image",
            "endpoint": "{{ ATLANTIC_REGISTRY }}",
            "path": "csi-node-driver-registrar",
    ])
    args.chartsPath.each {
        artifacts.add([
                "componentName": COMPONENT_NAME,
                "version": args.version,
                "type": "helm-chart",
                "endpoint": "{{ ASD_REPO }}",
                "path": it
        ])
    }
     artifacts.add([
            "componentName": COMPONENT_NAME,
            "version": args.version,
            "type": "file",
            "endpoint": "{{ ASD_REPO }}",
            "path": args.pathToFile,
     ])

    return common.toJSON(["componentVersion": args.version, "componentArtifacts": artifacts], true)
}

void publishCSIArtifactsToArtifactory(final Map<String, Object> args) {
    final String chartsBuildPath = "build/charts"
    ["baremetal-csi-plugin", "scheduler-extender"].each {
        sh("""
        helm package charts/${it}/ --set image.tag=${args.version} --version ${args.version} --destination ${chartsBuildPath}
    """)
    }

    files = common.findFiles("${chartsBuildPath}/*.tgz")
    List <String> charts = []
    files.each{ f ->
        charts.add("${ARTIFACTORY_CHARTS_PATH}/${args.version}/"+ f.getName())
        final String remoteName = f.getRemote()
        final String chartsPathToPublish = "${ARTIFACTORY_FULL_CHARTS_PATH}/${args.version}"
        common.publishFileToArtifactory(remoteName, chartsPathToPublish, common.ARTIFACTORY.ATLANTIC_PUBLISH_CREDENTIALS_ID)
    }

    final String pathToPublish = "${ARTIFACTORY_COMPONENT_PATH}/${args.version}"
    final String pathToFile = "pkg/scheduler/openshift_patcher.sh"
    sh('''
        sed -i  \'s/.*IMAGE=.*/IMAGE=${args.version}/\' ${pathToFile}
    ''')
    file = common.findFiles("${pathToFile}")[0]
    final String remoteName = file.getRemote()
    common.publishFileToArtifactory(remoteName, pathToPublish, common.ARTIFACTORY.ATLANTIC_PUBLISH_CREDENTIALS_ID)
    final String text = this.getArtifactsJson([
            version: args.version,
            chartsPath: charts,
            pathToFile: pathToPublish + "/" + remoteName
    ])

    writeFile(file: "artifacts.json",
            text: text)

    common.publishFileToArtifactory("artifacts.json", pathToPublish, common.ARTIFACTORY.ATLANTIC_PUBLISH_CREDENTIALS_ID)
}


this
