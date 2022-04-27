# Protozoa
A simulation of organisms navigating their environment according to inherited traits and decision trees.
Rendered with [ebiten](https://github.com/hajimehoshi/ebiten)

![Screen Shot 2022-04-27 at 11 15 26 AM](https://user-images.githubusercontent.com/3377325/165582772-ddb19bd8-8610-48af-b735-26ffb8872434.png)


## Simulation Rules 
Each simulation run begins by generating a number of organisms and food items at random on a 2D grid. 
Each render cycle, organisms must choose an action (eat, move, turn, attack etc.) based on available information about their surroundings. Organisms that survive long enough can spawn nearly identical offspring, thus propagating successful traits and behaviors.

### Environment
The environment consists of a simple 2D wraparound grid where each location contains a ph value (0-10). These ph values play a large role in organism health, and are likewise affected by certain organism actions (ie. growth). 

Each cycle, ph values diffuse between neighboring grid locations at a regular rate, such that the whole environment will gradually approach a single ph value in the absence of organism activity.

Low ph (acidic) locations appear pink, high ph (alkaline) locations are green, and neutral locations (~5.0 ph) are black.

<img src="https://user-images.githubusercontent.com/3377325/165464843-372bce5d-d150-4ffd-89ac-138aaa45787d.png" width="300">

### Food

Food items are generated at a regular rate throughout the simulation run and will appear randomly where there is room to place them. Each food item is represented by a dark gray square and contains a value between 0 and 100, representing how much the food item contains. When an organism sees a food item directly ahead, it can choose to 'eat' it, subtracting some value from the food and adding it to its own health. If a food item's value is reduced to 0, it disappears from the grid. Conversely, when an organism's health is reduced to 0 it 'dies' and is immediately replaced with a food item, whose value is set equal to the organism's size at death.

Apart from feeding organisms, food items also prevent movement. Organisms and food items cannot occupy the same location, and an organism facing a food item directly ahead cannot move through it.

![Food Items](https://user-images.githubusercontent.com/3377325/165467819-fb51b843-5fe3-422c-adf3-21212d65b1e3.png)

### Organisms

Organisms are represented by colored squares of different sizes, and they perform actions in their environment according to a set of genetic traits and a single decision tree. 'Health' and 'energy' are the same thing for organisms, and an organism's actions (moving, eating, etc.) may reduce its own health by some small amount to represent the energy exertion needed to do them. Further, an organism unable to tolerate the ph of its location will also have its health reduced until conditions improve.

An organism's health is limited by its current size, so an organism of size 50 will have a max health of 50. When an organism gains more health than its size allows, it 'grows' in size by some fraction of the excess health gain.

#### Traits
Initial organisms are generated with random values for several 'genetic' traits, which are inherited by any spawned children:
  * **Color -** _generated from random hue, saturation, and brightness_
  * **MaxSize -** _the maximum size an organism can grow_
  * **SpawnHealth -** _the initial health given to a spawned child, which is also subtracted from the parent's health_
  * **MinHealthToSpawn -** _the minimum health required by the parent to spawn a new child (never less than SpawnHealth)_
  * **MinCyclesBetweenSpawns -** _the minimum number of cycles that must pass before the organism can produce another child_
  * **ChanceToMutateDecisionTree -** _The chance of the organism passing a mutated version of its decision tree onto each spawned child_
  * **IdealPh -** _The middle of the organism's ph tolerance range_
  * **PhTolerance -** _The absolute ph distance the organism can go from its ideal ph without adverse effects. (eg. An ideal ph of 3 and ph tolerance of 1 provide a tolerance zone of 2-4 ph)_
  * **PhEffect -** _the positive or negative factor the organism's growth has on the ph level of its location)_

#### Decision Trees
Each organism's behavior is governed by a decision tree composed of various conditions and actions. Organisms generated at simulation start are given randomly-selected trees built from these decision nodes, while spawned children inherit an identical or similar variation of their parents' decision tree. and chosen from the following:
##### Conditions
  * **CanMoveAhead -** _checks if the organism can move forward (false if a food item or another organism directly ahead)_
  * **IsRandomFiftyPercent -** _returns true if a randomly generated float is less than .5_
  * **IsFoodAhead -** _true if a food item directly ahead_
  * **IsFoodLeft -** _true if a food item lies 90 degrees to the left_
  * **IsFoodRight -** _true if a food item lies 90 degrees to the right_
  * **IsOrganismAhead -** _true if an organism directly ahead_
  * **IsOrganismLeft -** _true if an organism lies 90 degrees to the left_
  * **IsOrganismRight -** _true if an organism lies 90 degrees to the right_
  * **IsBiggerOrganismAhead -** _true if an organism of greater size directly ahead_
  * **IsRelatedOrganismAhead -** _true if an organism with a shared ancestor directly ahead_
  * **IsRelatedOrganismLeft -** _true if an organism with a shared ancestor lies 90 degrees to the left_
  * **IsRelatedOrganismRight -** _true if an organism with a shared ancestor lies 90 degrees to the right_
  * **IfHealthAboveFiftyPercent -** _true if organism's health values more than half its current size_
  * **IsHealthyPhHere -** _true if the ph level at current location is not harmful and allows chemosynthesis_
##### Actions
  * **Chemosynthesis -** _generates a small amount of health, if performed at a location with healthy ph_
  * **Eat -** _consumes a small amount of health to consume any food that lies directly ahead_
  * **Move -** _consumes a small amount of health to move forward, if no food or organism directly ahead_
  * **TurnLeft --** _consumes a small amount of health to turn 90 degrees left_
  * **TurnRight -** _consumes a small amount of health to turn 90 degrees right_
  * **Attack -** _consumes a large amount of health to reduce the health of any organism directly ahead_
  * **Feed -** _transfers a small amount of health to any organism directly ahead- deposits this amount as food if no organism ahead_

#### Display
Clicking on an organism in the simulation grid will display its traits and decision tree in the left-hand panel, as shown:

![Screen Shot 2022-04-26 at 9 14 18 PM](https://user-images.githubusercontent.com/3377325/165596847-a73b1ae0-5ad4-4bf0-96c2-fa8479a3fb48.png)  

As printed, each conditional statement (eg. "If Can Move Ahead") is followed by a line that splits into two branches. The first, top-most branch is the logic the organism will follow if the checked condition returns true. The second, bottom branch will evaluate if the condition returns false. All decision tree nodes evaluated in the previous cycle are followed by "◀◀". Thus, the example decision tree shows - in the previous cycle - the selected organism checked 'If Can Move Ahead' (true), checked 'If Food Right' (false), and so it chose the 'Move Ahead' action.

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
