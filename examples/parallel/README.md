# Parallel Workflows

This example uses two parallel child workflows to demonstrate paralleism.

```
Print: parent workflow start
Starting child workflow '900eef73-57c1-4891-a95b-b619f0e7f86d'
Activity message: Hello from co2 (1), received: Child workflow '900eef73-57c1-4891-a95b-b619f0e7f86d' says hello, got input: parent workflow start
Starting child workflow '3958c882-f98b-4458-8809-237810aaed28'
Activity message: Hello from co2 (2), received: Hello from co2 (1)
Activity message: Hello from co1, received: Child workflow '3958c882-f98b-4458-8809-237810aaed28' says hello, got input: parent workflow start
Workflow completed with output: Hello from co1Hello from co2 (2)
```

Uncomment the demo exports section to generate the diagram.

```mermaid
    flowchart TD
      Start[Start]
class Start startNode
      StartCw1[StartCw1]
class StartCw1 parallelNode
      StartCw2[StartCw2]
class StartCw2 parallelNode
      WaitAndMerge[WaitAndMerge]
      Start --&gt; StartCw1_PrintActivity
      subgraph StartCw1
          StartCw1_PrintActivity[PrintActivity]
class StartCw1_PrintActivity startNode
      end
      Start --&gt; StartCw2_ChildStep1
      subgraph StartCw2
          StartCw2_ChildStep1[ChildStep1]
class StartCw2_ChildStep1 startNode
          StartCw2_ChildStep2[ChildStep2]
          StartCw2_ChildStep1 --&gt; StartCw2_ChildStep2
      end
      StartCw1_PrintActivity --&gt; WaitAndMerge
      StartCw2_ChildStep2 --&gt; WaitAndMerge
classDef startNode fill:#9f6,stroke:#333,stroke-width:4px;
classDef parallelNode fill:#6cf,stroke:#333,stroke-width:2px;
```