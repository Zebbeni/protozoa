# Protozoa
An experiment to simulate simple organisms navigating their environment through binary decision trees. Each organism maintains a library of decision trees that it uses and modifies as it seeks the algorithm most successful in improving its health.

Rendered using [ebiten](https://github.com/hajimehoshi/ebiten)

## Demo

![Demo](https://andrewsrandom.s3.us-west-2.amazonaws.com/Github+Media/protozoa_demo.gif)

## Simulation rules

- Gray squares represent food
- Colored squares represent Organisms
  - Organisms change colors according to the decision tree they are following
  - Organisms have health between 0 and 100. They die when their health reaches 0.
  - Organisms become food when they die.
  - Organisms lose health over time but lose health faster when moving.
  - Organisms gain up to 100 health when they eat a food in front of them
  - Organisms can attack an organism in front of them, decreasing their health.
- Decision Trees may contain any mix of the following conditions:
    - Can Move Ahead
    - If FiftyFifty
    - Is Food Ahead
    - Is Food Left
    - Is Food Right
    - Is Organism Ahead
    - Is Organism Left
    - Is Organism Right
    - Is Bigger Organism Ahead
    - Is Bigger Organism Left
    - Is Bigger Organism Right
    - Is Smaller Organism Ahead
    - Is Smaller Organism Left
    - Is Smaller Organism Right
    - Is Related Organism Ahead
    - Is Related Organism Left
    - Is Related Organism Right
    - If Health Above 50%
- Decision Trees may contain any mix of the following actions:
    - Be Idle
    - Attack
    - Feed
    - Eat
    - Move Ahead
    - Turn Left
    - Turn Right
  
# Setup
```
go get
go run main.go
```

# Config
Simulation config values can be overridden by providing json config files at runtime.
- Print the default settings as json:
```
go run main.go -dump-config
```
- Run with custom overrides:
```
go run main.go -config=settings/small.json
```

# Run Headless
- Single trial:
```
go run main.go -headless
```
- Multiple trials:
```
go run main.go -headless -trials=10
```

# Test
```
go test test/utils_test.go
```
