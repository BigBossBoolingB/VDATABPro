import pickle
from dataclasses import dataclass

# This assumes that vdatabprot is installed and accessible
from vdatabprot.rol import write, read

@dataclass
class Pet:
    name: str
    species: str
    level: int
    element: str

def main():
    """
    The main function for the CritterCraft PoC.
    """
    # 1. Instantiate
    pet_alpha = Pet(name="Cyber-Wolf Alpha", species="Canine", level=1, element="Data")
    print(f"Original Pet: {pet_alpha}")

    # 2. Serialize
    pet_bytes = pickle.dumps(pet_alpha)

    # 3. Store
    vector_id = write(pet_bytes)
    print(f"Stored pet with vector ID: {vector_id}")

    # 4. Retrieve
    retrieved_bytes = read(vector_id)

    # 5. Deserialize
    retrieved_pet = pickle.loads(retrieved_bytes)
    print(f"Retrieved Pet: {retrieved_pet}")

    # 6. Verify
    assert pet_alpha == retrieved_pet
    print("Verification successful: Original and retrieved pets match.")

if __name__ == "__main__":
    main()
