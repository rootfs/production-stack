import os

import numpy as np
import pandas as pd
from matplotlib import pyplot as plt

QPS_RANGE = [0.1, 0.5, 0.9, 1.3, 1.7, 2.1, 2.5, 2.9, 3.3, 3.7, 4.1]
stack_results = []
qpses = []
for qps in QPS_RANGE:
    # round qps to 1 decimal
    q = round(qps, 1)
    file = f"stack_output_{q}.csv"
    if not os.path.exists(file):
        continue
    qpses += [q]
    # Read csv file
    data = pd.read_csv(file)["ttft"].tolist()
    stack_results.append(sum(data) / len(data))
print("vLLM Production Stack TTFT", stack_results)

plt.plot(
    qpses,
    stack_results,
    label="Production Stack",
    marker="x",
    color="blue",
    linewidth=2,
    markersize=8,
)
aibrix_results = []
qpses = []
for qps in QPS_RANGE:
    # round qps to 1 decimal
    q = round(qps, 1)

    file = f"aibrix_output_{q}.csv"
    if not os.path.exists(file):
        continue
    qpses += [q]
    # Read csv file
    data = pd.read_csv(file)["ttft"].tolist()
    aibrix_results.append(sum(data) / len(data))
print("AIBrix TTFT", aibrix_results)
plt.plot(
    qpses,
    aibrix_results,
    label="Aibrix",
    marker="o",
    color="red",
    linewidth=2,
    markersize=8,
)

native_results = []
qpses = []
for qps in QPS_RANGE:
    # round qps to 1 decimal
    q = round(qps, 1)
    file = f"naive_output_{q}.csv"
    if not os.path.exists(file):
        continue
    # Read csv file
    data = pd.read_csv(file)["ttft"].tolist()
    native_results.append(sum(data) / len(data))
    qpses += [q]
plt.plot(qpses, native_results, label="Native K8S", color="orange", marker="v")
print("Naive k8s TTFT", native_results)

plt.xlim(left=0)
plt.ylim(top=10)
plt.xlabel("QPS")
plt.ylabel("Average Time to First Token (s)")
plt.title("Average Time to First Token vs QPS")
plt.legend()
plt.savefig("figure.png")
