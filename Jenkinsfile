// Deployable services (services/template is a scaffold, never deployed).
// No `def`: script-binding globals are visible inside every script {} block.
ALL_SERVICES = ['asset', 'finance', 'profile', 'project', 'purchase', 'resource']

// Paths (relative to repo root) whose change means every service must rebuild.
SHARED_PATHS = ['KAMIS-BE-GO/go.mod', 'KAMIS-BE-GO/go.sum', 'KAMIS-BE-GO/pkg/']

pipeline {
    agent any

    triggers {
        // Fired by the GitHub webhook -> https://<jenkins>/github-webhook/
        githubPush()
    }

    options {
        // Each build pushes to the gitops repo; overlapping pushes would race.
        disableConcurrentBuilds()
    }

    parameters {
        choice(
            name: 'SERVICE',
            // 'auto' first = default, which is what webhook-triggered builds get.
            choices: ['auto', 'asset', 'finance', 'profile', 'project', 'purchase', 'resource', 'all'],
            description: 'auto = build services changed since last successful build'
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
        stage('Detect Changes') {
            steps {
                script {
                    def services = []

                    if (params.SERVICE == 'all') {
                        services = ALL_SERVICES
                    } else if (params.SERVICE != 'auto') {
                        services = [params.SERVICE]
                    } else {
                        def base = env.GIT_PREVIOUS_SUCCESSFUL_COMMIT
                        // No baseline, or baseline no longer in history (force-push): rebuild all.
                        def baseOk = base && sh(
                            script: "git cat-file -e ${base}^{commit}",
                            returnStatus: true
                        ) == 0

                        if (!baseOk) {
                            echo "No usable previous successful commit; building all services"
                            services = ALL_SERVICES
                        } else {
                            def changed = sh(
                                script: "git diff --name-only ${base} ${env.GIT_COMMIT}",
                                returnStdout: true
                            ).trim().split('\n')
                            echo "Changed files since ${base.take(7)}:\n${changed.join('\n')}"

                            def sharedHit = false
                            for (f in changed) {
                                for (p in SHARED_PATHS) {
                                    if (f == p || f.startsWith(p)) { sharedHit = true }
                                }
                            }

                            if (sharedHit) {
                                echo "Shared code changed; building all services"
                                services = ALL_SERVICES
                            } else {
                                for (svc in ALL_SERVICES) {
                                    for (f in changed) {
                                        if (f.startsWith("KAMIS-BE-GO/services/${svc}/") && !services.contains(svc)) {
                                            services << svc
                                        }
                                    }
                                }
                            }
                        }
                    }

                    env.SERVICES = services.join(',')
                    env.TAG = env.GIT_COMMIT ? env.GIT_COMMIT.take(7) : 'manual'
                    echo "Services to build: ${env.SERVICES ?: '(none)'} @ ${env.TAG}"
                }
            }
        }

        stage('Test') {
            steps {
                dir('KAMIS-BE-GO') {
                    sh 'go test ./...'
                }
            }
        }

        stage('Build & Push') {
            when { expression { (env.SERVICES ?: '') != '' } }
            steps {
                script {
                    for (svc in env.SERVICES.split(',')) {
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
            when { expression { (env.SERVICES ?: '') != '' } }
            steps {
                withCredentials([string(credentialsId: 'gitops-token', variable: 'TOKEN')]) {
                    script {
                        sh '''#!/usr/bin/env bash
                            set -euo pipefail
                            rm -rf gitops
                            git clone --depth 1 "https://x-access-token:${TOKEN}@${GITOPS_REPO}" gitops
                        '''

                        for (svc in env.SERVICES.split(',')) {
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
                            git commit -am "deploy kamis ${env.SERVICES} ${env.TAG} (build ${env.BUILD_NUMBER})"
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
