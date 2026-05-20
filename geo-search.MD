# Geographical Search Results

## Current state
1. We only save docs in the tables and search based on user query and `doc.text`. 
2. The ranking for the docs is done by the BM25 algorithm which is currently purely text based


## Future usecase: extend on geographical domain
Ex: I am travelling in Delhi, India randomly, and I want to find a place which is *"Peaceful and less crowded"*.
Our app will have the description of all the places registered on it. 
```
description = [
0: "Good palce to visit with friend",
1: "crowd was less",
2: "highly crowded",
3: "Noisy but lively",
4: "great for night experience",
5: "peaceful place for travellers",
6: "extremely peaceful vibes",
7: "less crowded and peaceful"
....
]
```
Based on the above description the 1st, 5th, 6th and 7th place seems relevant for us. But now 5th and 6th are in USA and Australia. 1st place is in Bangalore, only 7th is in India. **So it doesn't make any sense to show all of them to the users**. 

One thing is thus clear we must include the distance from the current user location as well while giving the search results. Otherwise user might get too many irrelevant results if we only use text

## Feature description
1. Search results should be sorted by **relevancy (BM25) + distance** instead of just relevance. We will sort it by `k.text-relevancy + (1-k).distance` or any other metrics if needed.
2. Give user the option to **set the radius**


## Implementation
### Pre-requestie
1. Start storing `location (lat, long)` along with `description-text` in the tables
2. Code changes obviously :>)

