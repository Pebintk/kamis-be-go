pipeline {
    agent any

    parameters {
        choice(
            name: 'SERVICE',
            choices: ['asset', 'finance', 'profile', 'project', 'purchase', 'resource', 'all'],
            description: 'KAMIS service to build and deploy'
        )
    }

    environment {
        PATH       = "/usr/local/go/bin:${env.PATH}"
        AR_HOST    = "us-central1-docker.pkg.dev"
        PROJECT_ID = "ops-lab-506804"
        AR_REPO    = "lab-images"
        BASE_IMAGE = "us-central1-docker.pkg.dev/ops-lab-506804/lab-images/kamis"
        GITOPS_REPO = "github.com/Pebintk/lab-git-ops.git"
    }

    stages {
        stage('Test') {
            steps {
                dir('KAMIS-BE-GO') {
                    sh 'go test ./...'
                }
            }
        }

        stage('Build & Push') {
            steps {
                script {
                    env.TAG = env.GIT_COMMIT ? env.GIT_COMMIT.take(7) : 'manual'
                    def services = params.SERVICE == 'all' ?
                        ['asset', 'finance', 'profile', 'project', 'purchase', 'resource'] :
                        [params.SERVICE]

                    for (svc in services) {
                        def img = "${env.AR_HOST}/${env.PROJECT_ID}/${env.AR_REPO}/kamis-${svc}"
                        echo "Building kamis-${svc}:${env.TAG}..."

                        dir('KAMIS-BE-GO') {
                            sh "docker build -f services/${svc}/Dockerfile --build-arg SERVICE=${svc} -t ${img}:${env.TAG} ."
                            sh "docker push ${img}:${env.TAG}"
                        }
                    }
                }
            }
        }

        stage('Bump Manifest') {
            steps {
                withCredentials([string(credentialsId: 'gitops-token', variable: 'TOKEN')]) {
                    script {
                        def services = params.SERVICE == 'all' ?
                            ['asset', 'finance', 'profile', 'project', 'purchase', 'resource'] :
                            [params.SERVICE]

                        sh '''#!/usr/bin/env bash
                            set -euo pipefail
                            rm -rf gitops
                            git clone --depth 1 "https://x-access-token:${TOKEN}@${GITOPS_REPO}" gitops
                        '''

                        for (svc in services) {
                            def img = "${env.AR_HOST}/${env.PROJECT_ID}/${env.AR_REPO}/kamis-${svc}"
                            def overlay = "manifests/kamis/overlays/dev/${svc}"

                            sh """#!/usr/bin/env bash
                                set -euo pipefail
                                cd gitops/${overlay}
                                kustomize edit set image ${env.BASE_IMAGE}=${img}:${env.TAG}
                            """
                        }

                        sh """#!/usr/bin/env bash
                            set -euo pipefail
                            cd gitops
                            if git diff --quiet; then
                                echo "Manifests already at ${env.TAG}, nothing to commit"
                                exit 0
                            fi

                            git config user.email "jenkins@lab.local"
                            git config user.name  "jenkins-ci"
                            git commit -am "deploy kamis ${params.SERVICE} ${env.TAG} (build ${env.BUILD_NUMBER})"
                            git push origin HEAD:main
                        """
                    }
                }
            }
        }
    }

    post {
        always {
            sh 'docker image prune -f'
        }
    }
}
