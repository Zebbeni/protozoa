# Concept
This is a experiment to simulate organisms that change their decision-making algorithm to become more successful at surviving in their environment. 

All organisms act according to a binary decision tree, which they mutate or switch according to whichever decision tree in use appears to have the highest success rate in maintaining good organism health.

## Demo
![Protozoa Demo](https://s3-us-west-2.amazonaws.com/andrewsrandom/Github+Media/protozoa.gif)

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
    - If Can Move Ahead
    - If Food Ahead
    - If Food Left
    - If Food Right
    - If Organism Ahead
    - If Organism Left
    - If Organism Right
- Decision Trees may contain any mix of the following actions:
    - Be Idle
    - Attack
    - Eat
    - Move Ahead
    - Turn Left
    - Turn Right

# Setup
```
go get github.com/hajimehoshi/ebiten/...
go run main.go
```

# Test
```
go test test/utils_test.go
```
