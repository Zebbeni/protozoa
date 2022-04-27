# Protozoa
A simulation of single-celled organisms navigating a simple environment via decision trees and inherited traits.

Rendered using [ebiten](https://github.com/hajimehoshi/ebiten)

## Demo
![Demo](https://user-images.githubusercontent.com/3377325/165461211-7025ac40-121f-4fbf-a068-e9eabc054dac.gif)

## Simulation Rules 
Each simulation run starts by generating a number of organisms and food items, placed randomly on a 2D grid. 
Each render cycle, organisms  choose an action (eat, move, turn, attack etc.) based on available information about their surroundings. 

Organisms whose decisions allow them to survive spawn children that inherit their decision trees and genetic traits, propagating successful 'species' and behaviors. 

### Environment
The environment consists of a simple 2D wraparound grid where each location contains a ph value (0-10). These ph values play a large role in organism health, and are likewise affected by certain organism actions (ie. growth). 

'Acidic', low ph locations are shown as pink, while 'Alkaline', high ph locations appear green. Neutral ph locations (~5.0 ph) are drawn black.

Each cycle, ph values diffuse between neighboring grid locations at a regular rate, such that the whole environment will gradually approach a single ph value in the absence of organism activity.

### Food

Food items are represented by 

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
