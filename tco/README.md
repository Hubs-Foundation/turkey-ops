# **t**urkey **c**luster **o**orchestrator

### contains:
- bash to create `db server` (RDS-postgres) and k8s cluster (EKS) in AWS
- yamls to deploy turkey

### design and usage:
- wrapped in github action pipeline
- human can run the pipeline by hand
- codes can run the pipeline by webhook events
  - https://docs.github.com/en/actions/learn-github-actions/events-that-trigger-workflows

