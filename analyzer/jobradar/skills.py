import re
from pathlib import Path

# written form (matched case-insensitively) -> canonical skill name.
# The canonical set is set(ALIASES.values()); there is no separate list to keep in sync.
#
# Curated "main" skills, not an exhaustive taxonomy. Single-letter languages (C, R) are
# left out on purpose: the word-boundary matcher would catch the C in "C++"/"C#" and R
# everywhere, which needs longest-match. Some names are also English words ("go",
# "spring", "express"), so counts over-report a little. confidence is a raw mention
# count, not normalized: the models use skills as a set (jobradar/match.py), not counts.
ALIASES = {
    # Languages
    "python": "Python",
    "go": "Go",
    "golang": "Go",
    "java": "Java",
    "javascript": "JavaScript",
    "js": "JavaScript",
    "typescript": "TypeScript",
    "ts": "TypeScript",
    "c++": "C++",
    "cpp": "C++",
    "c#": "C#",
    "csharp": "C#",
    "ruby": "Ruby",
    "php": "PHP",
    "rust": "Rust",
    "kotlin": "Kotlin",
    "swift": "Swift",
    "scala": "Scala",
    "sql": "SQL",
    "bash": "Bash",
    "shell": "Shell",
    "perl": "Perl",
    "elixir": "Elixir",
    "haskell": "Haskell",
    "dart": "Dart",
    "lua": "Lua",
    "objective-c": "Objective-C",
    "matlab": "MATLAB",
    "clojure": "Clojure",
    # Frontend
    "react": "React",
    "reactjs": "React",
    "angular": "Angular",
    "vue": "Vue",
    "vuejs": "Vue",
    "svelte": "Svelte",
    "next.js": "Next.js",
    "nextjs": "Next.js",
    "html": "HTML",
    "css": "CSS",
    "tailwind": "Tailwind",
    "tailwindcss": "Tailwind",
    # Backend / frameworks / APIs
    "node.js": "Node.js",
    "nodejs": "Node.js",
    "django": "Django",
    "flask": "Flask",
    "fastapi": "FastAPI",
    "spring": "Spring",
    "rails": "Rails",
    "express": "Express",
    ".net": ".NET",
    "dotnet": ".NET",
    "laravel": "Laravel",
    "graphql": "GraphQL",
    "grpc": "gRPC",
    "restful": "REST",
    "rest api": "REST",
    # Databases
    "postgresql": "PostgreSQL",
    "postgres": "PostgreSQL",
    "mysql": "MySQL",
    "mongodb": "MongoDB",
    "mongo": "MongoDB",
    "redis": "Redis",
    "sqlite": "SQLite",
    "cassandra": "Cassandra",
    "elasticsearch": "Elasticsearch",
    "dynamodb": "DynamoDB",
    "oracle": "Oracle",
    "mariadb": "MariaDB",
    # Cloud / DevOps
    "aws": "AWS",
    "gcp": "GCP",
    "google cloud": "GCP",
    "azure": "Azure",
    "docker": "Docker",
    "kubernetes": "Kubernetes",
    "k8s": "Kubernetes",
    "terraform": "Terraform",
    "ansible": "Ansible",
    "jenkins": "Jenkins",
    "ci/cd": "CI/CD",
    "ci-cd": "CI/CD",
    "cicd": "CI/CD",
    "github actions": "GitHub Actions",
    "gitlab ci": "GitLab CI",
    "prometheus": "Prometheus",
    "grafana": "Grafana",
    "helm": "Helm",
    "nginx": "Nginx",
    "kafka": "Kafka",
    "rabbitmq": "RabbitMQ",
    # Data / ML
    "pandas": "pandas",
    "numpy": "NumPy",
    "scikit-learn": "scikit-learn",
    "sklearn": "scikit-learn",
    "tensorflow": "TensorFlow",
    "pytorch": "PyTorch",
    "spark": "Spark",
    "airflow": "Airflow",
    # Tools / VCS / OS
    "git": "Git",
    "github": "GitHub",
    "gitlab": "GitLab",
    "bitbucket": "Bitbucket",
    "linux": "Linux",
    "jira": "Jira",
}


def skills_extractor(text: str, skills: dict[str, str]) -> dict[str, float]:
    skills_found: dict[str, float] = {}
    for s, v in skills.items():
        found = len(re.findall(rf"(?<!\w)({re.escape(s)})(?!\w)", text, re.IGNORECASE))
        if found != 0:
            skills_found[v] = skills_found.get(v, 0) + found
    return skills_found


if __name__ == "__main__":
    cv_path = Path(__file__).resolve().parents[1] / "cv.md"
    cv_text = cv_path.read_text(encoding="utf-8")
    skills_found = skills_extractor(cv_text, ALIASES)
    print(f"cv.md: {len(cv_text)} chars · dictionary: {len(ALIASES)} skills")
    print("Skills found:")
    for skill, weight in skills_found.items():
        print(f"{skill}: {weight}")


# The canonical skills that are programming languages (or a language's runtime): what a
# job is written in, as opposed to the tools around it.
LANGUAGES = frozenset(
    {
        "C#",
        "C++",
        "Elixir",
        "Go",
        "Java",
        "JavaScript",
        "Kotlin",
        "Node.js",
        "PHP",
        "Python",
        "Ruby",
        "Rust",
        "Scala",
        "Swift",
        "TypeScript",
    }
)
