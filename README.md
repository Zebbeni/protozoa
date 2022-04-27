# Protozoa
A simulation of organisms navigating their environment according to inherited traits and decision trees.

Rendered with [ebiten](https://github.com/hajimehoshi/ebiten)

## Demo
![Demo](https://user-images.githubusercontent.com/3377325/165461211-7025ac40-121f-4fbf-a068-e9eabc054dac.gif)

## Simulation Rules 
Each simulation run begins by generating a number of organisms and food items at random on a 2D grid. 
Each render cycle, organisms must choose an action (eat, move, turn, attack etc.) based on available information about their surroundings. Organisms that survive long enough can spawn nearly identical offspring, thus propagating successful traits and behaviors.

### Environment
The environment consists of a simple 2D wraparound grid where each location contains a ph value (0-10). These ph values play a large role in organism health, and are likewise affected by certain organism actions (ie. growth). 

Each cycle, ph values diffuse between neighboring grid locations at a regular rate, such that the whole environment will gradually approach a single ph value in the absence of organism activity.

Low ph (acidic) locations appear pink, high ph (alkaline) locations are green, and neutral locations (~5.0 ph) are black.

<img src="https://user-images.githubusercontent.com/3377325/165464843-372bce5d-d150-4ffd-89ac-138aaa45787d.png" width="300">

### Food

Food items are generated at a regular rate throughout the simulation run and will appear randomly where there is room to place them. Each food item is represented by a dark gray square and contains a value between 0 and 100, representing how much the food item contains. When an organism sees a food item directly ahead, it can choose to 'eat' it, subtracting some value from the food and adding it to its own health. If a food item's value is reduced to 0, it disappears from the grid. Conversely, when an organism dies it is immediately replaced with a food item, whose value is set equal to the organism's size at death.

Apart from feeding organisms, food items also prevent movement. Organisms and food items cannot occupy the same location, and an organism facing a food item directly ahead cannot move through it.

![Food Items](https://user-images.githubusercontent.com/3377325/165467819-fb51b843-5fe3-422c-adf3-21212d65b1e3.png)

### Organisms

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
    - Is Smaller Organism Ahead
    - Is Related Organism Ahead
    - Is Related Organism Left
    - Is Related Organism Right
    - If Health Above 50%
- Decision Trees may contain any mix of the following actions:
    - Chemosynthesis
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
