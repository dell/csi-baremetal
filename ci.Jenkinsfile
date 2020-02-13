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
            csiTag                     : params.CSI_TAG,
            runMode                    : '',
            slackChannel               : '',
            harborProject              :'ecs',
    ]
    String csiTag = args.csiTag
    boolean testResultSuccess = false
    String output = ''
    final String RUN_MODE_MASTER = 'master'
    final String RUN_MODE_CUSTOM = 'custom'
    final String registry = "10.244.120.194:8085/atlantic"  // asdrepo.isus.emc.com:8085/atlantic
    String containerId = ''
    boolean allGood = false
    common.node(label: common.JENKINS_LABELS.FLEX_CI, time: 180) {
        String workspace = pwd()
        try {
            stage('Prepare iscsi') {
                sh("zypper install -y open-iscsi")
                containerId = sh(script: "docker run -d --net=host ${registry}/itt:latest itt -p 127.0.0.1 -t 127.0.0.1:5230 -d /opt/emc/etc/itt/dev_ecs-test.xml",
                                 returnStdout: true);
                sh("""
                   service iscsid start
                   iscsiadm --mode discovery --type=sendtargets --portal 127.0.0.1
                   iscsiadm --mode node --portal 127.0.0.1:3260 --login
                """)
            }
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
                stage('Image Pulling') {
                     //e2e tests need busybox:1.29 for testing pods
                     sh("""
                        docker pull ${registry}/csi-provisioner:v1.2.2
                        docker pull ${registry}/csi-attacher:v1.0.1
                        docker pull busybox:1.29
                        docker pull ${registry}/csi-node-driver-registrar:v1.0.1-gke.0
                        docker pull ${registry}/baremetal-csi-plugin-controller:${csiTag}
                        docker pull ${registry}/baremetal-csi-plugin-node:${csiTag}
                        docker pull ${registry}/baremetal-csi-plugin-hwmgr:${csiTag}
                        """);
                }
                 //E2E tests can't work with helm, so we need to provide prepared yaml files for it
                 stage('Prepare YAML for e2e tests'){
                     dir('baremetal-csi-plugin'){
                         sh("helm template charts/baremetal-csi-plugin/ "+
                             "--output-dir /tmp --set image.tag=${csiTag} "+
                             "--set image.pullPolicy=IfNotPresent")
                     }
                 }
                 stage('Create Kind cluster') {
                      dir('baremetal-csi-plugin'){
                           sh("""
                              kind create cluster --kubeconfig /root/.kube/config --config config.yaml
                              kind load docker-image ${registry}/csi-provisioner:v1.2.2
                              kind load docker-image ${registry}/csi-attacher:v1.0.1
                              kind load docker-image ${registry}/csi-node-driver-registrar:v1.0.1-gke.0
                              kind load docker-image ${registry}/baremetal-csi-plugin-controller:${csiTag}
                              kind load docker-image ${registry}/baremetal-csi-plugin-node:${csiTag}
                              kind load docker-image ${registry}/baremetal-csi-plugin-hwmgr:${csiTag}
                              kind load docker-image busybox:1.29
                              kubectl config set-context \"kind-kind\"
                              """)
                      }
                 }
                 stage('E2E testing') {
                      dir('baremetal-csi-plugin'){
                           output = sh(script: 'go run test/e2e/baremetal_e2e.go -ginkgo.v -ginkgo.progress --kubeconfig=/root/.kube/config', returnStdout: true);
                           println output
                           if (!(output.contains("FAIL"))){
                                testResultSuccess = true
                           }
                      }
                 }
                 stage('Delete kind cluster') {
                     dir('baremetal-csi-plugin'){
                         sh('kind delete cluster')
                     }
                 }
                 stage('Image retagging'){
                     if (testResultSuccess && (args.runMode == RUN_MODE_MASTER)){
                          harbor.retagCSIImages(args.harborProject, args.csiTag, 'latest')
                          String repo = 'baremetal-csi-plugin'
                          DockerImage sourceImage = new DockerImage(registry: DockerRegistries.ASDREPO_ECS_REGISTRY, repo: repo, tag: args.csiTag)
                          DockerImage newImage = new DockerImage(registry: DockerRegistries.ASDREPO_ECS_REGISTRY, repo: repo, tag: 'latest')
                          sh(docker.getRetagCommand(sourceImage, newImage))
                     }
                 }
            }
            allGood=true
        }
        finally {
            sh("""
               iscsiadm --mode node --portal 127.0.0.1:3260 --logout
               iscsiadm --mode node --portal 127.0.0.1:3260 --op delete
               docker rm -f ${containerId}
            """)
            if (!testResultSuccess || !allGood) {
                common.setBuildFailure()
            }
            common.slackSend(channel: args.slackChannel)
        }
    }
}
this
