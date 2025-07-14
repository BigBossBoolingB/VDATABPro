# Interlink & Context Protocol (ICP) Architecture

## 1. Overview

The Interlink & Context Protocol (ICP) is the fourth and final layer of the VDataBProt stack. It is the intelligence layer, responsible for identifying and leveraging relationships between `DataStateVector` objects to anticipate data needs and dramatically accelerate performance. The ICP transforms VDataBProt from a static storage system into a dynamic, self-optimizing ecosystem.

## 2. The "Link" Primitive

The atomic unit of the ICP is the **Link**. A Link represents a discovered relationship between two vectors. It is a lightweight data structure that will be stored and managed by the ICP.

### Link Data Structure

A Link object will have the following structure:

```
{
  "source_vector_id": "string",
  "target_vector_id": "string",
  "context_type": "enum",
  "strength_score": "float"
}
```

- **`source_vector_id`**: The ID of the vector from which the link originates.
- **`target_vector_id`**: The ID of the vector to which the link points.
- **`context_type`**: An enumeration defining the nature of the discovered relationship. This provides the "why" behind the link.
  - `Causal`: `target_vector_id` was created or modified as a direct result of an operation on `source_vector_id`.
  - `AccessedWithin`: `target_vector_id` was accessed within a short time window (e.g., < 5 seconds) after `source_vector_id`.
  - `ContentSimilarity`: (Future Scope) The reconstituted data of the two vectors share a high degree of semantic or structural similarity.
- **`strength_score`**: A floating-point number between 0.0 and 1.0 that quantifies the confidence or importance of the link. This score will be dynamic, allowing it to be reinforced by repeated discovery or decay over time if the relationship is no longer observed.

## 3. Discovery Heuristics

The ICP will rely on a set of discovery heuristics, run by the DIEE's "Anti-Entropy Analysis" function, to find and create Links.

### Heuristic 1: Temporal Proximity

The primary initial heuristic will be based on analyzing access logs.

- **Mechanism**: The ROL will be instrumented to log every `read` and `write` operation, including the vector ID and a high-precision timestamp.
- **Analysis**: The DIEE will periodically scan these logs. When it detects that two distinct vectors are consistently accessed within a predefined time window (e.g., 5 seconds) by the same process or user session, it will generate an `AccessedWithin` Link between them.
- **Strength Calculation**: The `strength_score` will be initialized based on the frequency of the co-occurrence and will be reinforced with each subsequent observation.

### Heuristic 2: Content-Based Analysis (Future Scope)

This heuristic is a placeholder for a more advanced, AI-driven analysis capability.

- **Mechanism**: Future versions of the DIEE will incorporate machine learning models capable of understanding the content of reconstituted data.
- **Analysis**: The DIEE could, for example, compare document embeddings, image feature vectors, or other content-based representations to identify semantic relationships.
- **Strength Calculation**: The `strength_score` would be derived from the model's confidence score in the similarity between the two pieces of content.

## 4. Caching & Pre-fetching Mechanism

The ultimate purpose of the ICP is to enable intelligent pre-fetching. This will be integrated directly into the ROL.

- **Trigger**: When the ROL receives a `read(vector_id)` request.
- **Logic**:
  1. The ROL retrieves the primary `DataStateVector` from the persistent storage.
  2. It then queries the ICP for any strong Links (`strength_score` > a configurable threshold, e.g., 0.8) originating from this `vector_id`.
  3. For each strong Link found, the ROL will issue an asynchronous request to fetch the `target_vector_id`'s `DataStateVector` from the persistent store.
  4. These pre-fetched vectors will be placed into a high-speed, in-memory cache.
- **Cache Implementation**: For the initial prototype, this will be a simple in-memory dictionary (a `dict` in Python). For a production system, this would be replaced by a more robust caching solution like Redis or Memcached.
- **Cache Hit**: When a subsequent `read` request is made for a vector ID that is already in the cache, the ROL will serve it directly from memory, bypassing the slower persistent storage and dramatically reducing latency.
